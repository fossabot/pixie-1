/*
 * Copyright 2018- The Pixie Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

#include <gtest/gtest.h>
#include <set>

#include "src/common/testing/testing.h"
#include "src/stirling/bpf_tools/bcc_wrapper.h"
#include "src/stirling/source_connectors/perf_profiler/symbolizer.h"

namespace test {
// foo() & bar() are not used directly, but in this test,
// we will find their symbols using the device under test, the symbolizer.
void foo() { LOG(INFO) << "foo()."; }

void bar() { LOG(INFO) << "bar()."; }
}  // namespace test

namespace px {
namespace stirling {

using ::testing::Contains;

class SymbolCacheTest : public ::testing::Test {
 protected:
  void SetUp() override {
    PL_CHECK_OK(bcc_.InitBPFProgram(kProgram));
    bcc_symbolizer_ = std::make_unique<ebpf::BPFStackTable>(bcc_.GetStackTable("bcc_symbolizer"));
  }

  const std::string_view kProgram = "BPF_STACK_TRACE(bcc_symbolizer, 16);";
  bpf_tools::BCCWrapper bcc_;
  std::unique_ptr<ebpf::BPFStackTable> bcc_symbolizer_;

  const uintptr_t kFooAddr = reinterpret_cast<uintptr_t>(&test::foo);
  const uintptr_t kBarAddr = reinterpret_cast<uintptr_t>(&test::bar);
};

TEST_F(SymbolCacheTest, Lookup) {
  SymbolCache sym_cache(getpid(), bcc_symbolizer_.get());

  SymbolCache::LookupResult result;

  result = sym_cache.Lookup(kFooAddr);
  EXPECT_EQ(result.hit, false);
  EXPECT_EQ(result.symbol, "test::foo()");

  result = sym_cache.Lookup(kFooAddr);
  EXPECT_EQ(result.hit, true);
  EXPECT_EQ(result.symbol, "test::foo()");

  result = sym_cache.Lookup(kBarAddr);
  EXPECT_EQ(result.hit, false);
  EXPECT_EQ(result.symbol, "test::bar()");
}

TEST_F(SymbolCacheTest, EvictOldEntries) {
  SymbolCache sym_cache(getpid(), bcc_symbolizer_.get());

  SymbolCache::LookupResult result;

  EXPECT_EQ(sym_cache.total_entries(), 0);
  EXPECT_EQ(sym_cache.active_entries(), 0);

  result = sym_cache.Lookup(kFooAddr);
  EXPECT_EQ(result.hit, false);
  EXPECT_EQ(result.symbol, "test::foo()");

  result = sym_cache.Lookup(kBarAddr);
  EXPECT_EQ(result.hit, false);
  EXPECT_EQ(result.symbol, "test::bar()");

  EXPECT_EQ(sym_cache.total_entries(), 2);
  EXPECT_EQ(sym_cache.active_entries(), 2);

  sym_cache.CreateNewGeneration();

  EXPECT_EQ(sym_cache.total_entries(), 2);
  EXPECT_EQ(sym_cache.active_entries(), 0);

  result = sym_cache.Lookup(kFooAddr);
  EXPECT_EQ(result.hit, true);
  EXPECT_EQ(result.symbol, "test::foo()");

  EXPECT_EQ(sym_cache.total_entries(), 2);
  EXPECT_EQ(sym_cache.active_entries(), 1);

  sym_cache.CreateNewGeneration();

  EXPECT_EQ(sym_cache.total_entries(), 1);
  EXPECT_EQ(sym_cache.active_entries(), 0);

  // Don't lookup test::foo() in this interval.
  // Should cause it to get evicted from the cache after the next trigger.

  sym_cache.CreateNewGeneration();

  EXPECT_EQ(sym_cache.total_entries(), 0);
  EXPECT_EQ(sym_cache.active_entries(), 0);

  sym_cache.CreateNewGeneration();

  EXPECT_EQ(sym_cache.total_entries(), 0);
  EXPECT_EQ(sym_cache.active_entries(), 0);

  result = sym_cache.Lookup(kFooAddr);
  EXPECT_EQ(result.hit, false);
  EXPECT_EQ(result.symbol, "test::foo()");

  EXPECT_EQ(sym_cache.total_entries(), 1);
  EXPECT_EQ(sym_cache.active_entries(), 1);
}

// Test the symbolizer with caching enabled and disabled.
TEST(SymbolizerTest, Basic) {
  // TODO(jps): consider splitting into 3 tests:
  // ... 1. for user symbolization
  // ... 2. for kernel symbolization
  // ... 3. for caching
  static constexpr auto kProbeSpecs = MakeArray<bpf_tools::KProbeSpec>(
      {{"getpid", bpf_tools::BPFProbeAttachType::kEntry, "syscall__get_pid"}});

  bpf_tools::BCCWrapper bcc_wrapper;

  const std::string_view kProgram =
      "#include <linux/socket.h>\n"
      "BPF_ARRAY(kaddr_array, u64, 1);"
      "int syscall__get_pid(struct pt_regs* ctx) {"
      "    int kIndex = 0;"
      "    u64* p = kaddr_array.lookup(&kIndex);"
      "    if( p == NULL ) {"
      "        return 0;"
      "    }"
      "    unsigned long long int some_kaddr = PT_REGS_IP(ctx);"
      "    *p = some_kaddr;"
      "    return 0;"
      "}";

  ASSERT_OK(bcc_wrapper.InitBPFProgram(kProgram));
  ASSERT_OK(bcc_wrapper.AttachKProbes(kProbeSpecs));

  ebpf::BPFArrayTable<uint64_t> kaddr_array = bcc_wrapper.GetArrayTable<uint64_t>("kaddr_array");

  // We will use our self pid for symbolizing symbols from within this process,
  // *and* we will trigger the kprobe that grabs a symbol from the kernel.
  const uint32_t pid = getpid();

  FLAGS_stirling_profiler_symcache = true;

  Symbolizer symbolizer;
  ASSERT_OK(symbolizer.Init());

  const struct upid_t this_upid = {.pid = pid, .start_time_ticks = 0};

  // Setup some address that we can symbolize:
  const uintptr_t foo_addr = (uintptr_t) & ::test::foo;
  const uintptr_t bar_addr = (uintptr_t) & ::test::bar;

  // Lookup the addresses for the first time. These should be cache misses.
  // We are placing each symbol lookup into its own scope to force us to
  // "re-lookup" the pid symbolizer function from inside of the symbolize instance.
  {
    auto symbolize = symbolizer.GetSymbolizerFn(this_upid);
    EXPECT_EQ(symbolize(foo_addr), "test::foo()");
    EXPECT_EQ(symbolizer.stat_accesses(), 1);
    EXPECT_EQ(symbolizer.stat_hits(), 0);
  }
  {
    auto symbolize = symbolizer.GetSymbolizerFn(this_upid);
    EXPECT_EQ(symbolize(bar_addr), "test::bar()");
    EXPECT_EQ(symbolizer.stat_accesses(), 2);
    EXPECT_EQ(symbolizer.stat_hits(), 0);
  }

  // Lookup the addresses a second time. We should get cache hits.
  {
    auto symbolize = symbolizer.GetSymbolizerFn(this_upid);
    EXPECT_EQ(symbolize(foo_addr), "test::foo()");
    EXPECT_EQ(symbolizer.stat_accesses(), 3);
    EXPECT_EQ(symbolizer.stat_hits(), 1);
  }

  {
    auto symbolize = symbolizer.GetSymbolizerFn(this_upid);
    EXPECT_EQ(symbolize(bar_addr), "test::bar()");
    EXPECT_EQ(symbolizer.stat_accesses(), 4);
    EXPECT_EQ(symbolizer.stat_hits(), 2);
  }

  // We see different kernel symbols on different hosts (not 100% sure why).
  // on our dev. host 'enigma' we see: __x64_sys_getpid
  // on our Jenkins test hosts we see: sys_getpid
  const std::set<std::string> possible_k_syms = {"__x64_sys_getpid", "__ia32_sys_getpid",
                                                 "sys_getpid"};

  uintptr_t kaddr = 0ULL;
  kaddr_array.get_value(0, kaddr);

  {
    auto symbolize = symbolizer.GetSymbolizerFn(profiler::kKernelUPID);
    EXPECT_THAT(possible_k_syms, Contains(std::string(symbolize(kaddr))));
    EXPECT_EQ(symbolizer.stat_accesses(), 5);
    EXPECT_EQ(symbolizer.stat_hits(), 2);
  }
  {
    auto symbolize = symbolizer.GetSymbolizerFn(profiler::kKernelUPID);
    EXPECT_THAT(possible_k_syms, Contains(std::string(symbolize(kaddr))));
    EXPECT_EQ(symbolizer.stat_accesses(), 6);
    EXPECT_EQ(symbolizer.stat_hits(), 3);
  }

  // This will flush the caches, access count & hit count will remain the same.
  // We will lookup the symbols and again expect a miss then a hit.
  symbolizer.FlushCache(this_upid);
  symbolizer.FlushCache(profiler::kKernelUPID);

  {
    auto symbolize = symbolizer.GetSymbolizerFn(this_upid);
    EXPECT_EQ(symbolize(foo_addr), "test::foo()");
    EXPECT_EQ(symbolizer.stat_accesses(), 7);
    EXPECT_EQ(symbolizer.stat_hits(), 3);
  }
  {
    auto symbolize = symbolizer.GetSymbolizerFn(this_upid);
    EXPECT_EQ(symbolize(bar_addr), "test::bar()");
    EXPECT_EQ(symbolizer.stat_accesses(), 8);
    EXPECT_EQ(symbolizer.stat_hits(), 3);
  }
  {
    auto symbolize = symbolizer.GetSymbolizerFn(profiler::kKernelUPID);
    EXPECT_THAT(possible_k_syms, Contains(std::string(symbolize(kaddr))));
    EXPECT_EQ(symbolizer.stat_accesses(), 9);
    EXPECT_EQ(symbolizer.stat_hits(), 3);
  }
  {
    auto symbolize = symbolizer.GetSymbolizerFn(this_upid);
    EXPECT_EQ(symbolize(foo_addr), "test::foo()");
    EXPECT_EQ(symbolizer.stat_accesses(), 10);
    EXPECT_EQ(symbolizer.stat_hits(), 4);
  }
  {
    auto symbolize = symbolizer.GetSymbolizerFn(this_upid);
    EXPECT_EQ(symbolize(bar_addr), "test::bar()");
    EXPECT_EQ(symbolizer.stat_accesses(), 11);
    EXPECT_EQ(symbolizer.stat_hits(), 5);
  }
  {
    auto symbolize = symbolizer.GetSymbolizerFn(profiler::kKernelUPID);
    EXPECT_THAT(possible_k_syms, Contains(std::string(symbolize(kaddr))));
    EXPECT_EQ(symbolizer.stat_accesses(), 12);
    EXPECT_EQ(symbolizer.stat_hits(), 6);
  }

  // After setting the caching flag to false,
  // we expect the cache stats to remain unchanged.
  FLAGS_stirling_profiler_symcache = false;
  {
    auto symbolize = symbolizer.GetSymbolizerFn(this_upid);
    EXPECT_EQ(symbolize(foo_addr), "test::foo()");
    EXPECT_EQ(symbolizer.stat_accesses(), 12);
    EXPECT_EQ(symbolizer.stat_hits(), 6);
  }
  {
    auto symbolize = symbolizer.GetSymbolizerFn(this_upid);
    EXPECT_EQ(symbolize(bar_addr), "test::bar()");
    EXPECT_EQ(symbolizer.stat_accesses(), 12);
    EXPECT_EQ(symbolizer.stat_hits(), 6);
  }
  {
    auto symbolize = symbolizer.GetSymbolizerFn(profiler::kKernelUPID);
    EXPECT_THAT(possible_k_syms, Contains(std::string(symbolize(kaddr))));
    EXPECT_EQ(symbolizer.stat_accesses(), 12);
    EXPECT_EQ(symbolizer.stat_hits(), 6);
  }

  // Test the feature that converts "[UNKNOWN]" into 0x<addr>.
  // Also make sure we get a cache hit (so set the cache flag back to true).
  FLAGS_stirling_profiler_symcache = true;
  {
    auto symbolize = symbolizer.GetSymbolizerFn(this_upid);
    EXPECT_EQ(symbolize(0x1234123412341234ULL), "0x1234123412341234");
    EXPECT_EQ(symbolizer.stat_accesses(), 13);
    EXPECT_EQ(symbolizer.stat_hits(), 6);
  }
  {
    auto symbolize = symbolizer.GetSymbolizerFn(this_upid);
    EXPECT_EQ(symbolize(0x1234123412341234ULL), "0x1234123412341234");
    EXPECT_EQ(symbolizer.stat_accesses(), 14);
    EXPECT_EQ(symbolizer.stat_hits(), 7);
  }
}

}  // namespace stirling
}  // namespace px
