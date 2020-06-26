#include "src/stirling/dynamic_tracing/code_gen.h"

#include "src/common/testing/testing.h"
#include "src/stirling/testing/testing.h"

namespace pl {
namespace stirling {
namespace dynamic_tracing {

using ::pl::stirling::dynamictracingpb::BPFHelper;
using ::pl::stirling::dynamictracingpb::MapStashAction;
using ::pl::stirling::dynamictracingpb::OutputAction;
using ::pl::stirling::dynamictracingpb::PhysicalProbe;
using ::pl::stirling::dynamictracingpb::Register;
using ::pl::stirling::dynamictracingpb::ScalarType;
using ::pl::stirling::dynamictracingpb::ScalarVariable;
using ::pl::stirling::dynamictracingpb::Struct;
using ::pl::stirling::dynamictracingpb::StructVariable;
using ::pl::stirling::dynamictracingpb::VariableType;
using ::testing::ElementsAre;
using ::testing::ElementsAreArray;
using ::testing::StrEq;

TEST(GenStructTest, Output) {
  Struct st;
  st.set_name("socket_data_event_t");

  Struct::Field* field = nullptr;

  field = st.add_fields();
  field->set_name("i32");
  field->mutable_type()->set_scalar(ScalarType::INT32);

  field = st.add_fields();
  field->set_name("i64");
  field->mutable_type()->set_scalar(ScalarType::INT64);

  field = st.add_fields();
  field->set_name("double_val");
  field->mutable_type()->set_scalar(ScalarType::DOUBLE);

  field = st.add_fields();
  field->set_name("msg");
  field->mutable_type()->set_scalar(ScalarType::VOID_POINTER);

  field = st.add_fields();
  field->set_name("str");
  field->mutable_type()->set_scalar(ScalarType::STRING);

  field = st.add_fields();
  field->set_name("attr");
  field->mutable_type()->set_struct_type("attr_t");

  ASSERT_OK_AND_THAT(GenStruct(st, /*indent_size*/ 4),
                     ElementsAre("struct socket_data_event_t {", "    int32_t i32;",
                                 "    int64_t i64;", "    double double_val;", "    void* msg;",
                                 "    char* str;", "    struct attr_t attr;", "};"));
}

TEST(GenVariableTest, Register) {
  ScalarVariable var;

  var.set_name("var");
  var.set_val_type(ScalarType::VOID_POINTER);
  var.set_reg(Register::SP);

  ASSERT_OK_AND_THAT(GenScalarVariable(var), ElementsAre("void* var = PT_REGS_SP(ctx);"));
}

TEST(GenVariableTest, MemoryVariable) {
  ScalarVariable var;

  var.set_name("var");
  var.set_val_type(ScalarType::INT32);

  auto* mem_var = var.mutable_memory();

  mem_var->set_base("sp");
  mem_var->set_offset(123);

  ASSERT_OK_AND_THAT(
      GenScalarVariable(var),
      ElementsAre("int32_t var;", "bpf_probe_read(&var, sizeof(int32_t), sp + 123);"));
}

TEST(GenVariableTest, Builtin) {
  ScalarVariable var;

  var.set_name("var");
  var.set_val_type(ScalarType::VOID_POINTER);

  var.set_builtin(BPFHelper::GOID);

  ASSERT_OK_AND_THAT(GenScalarVariable(var), ElementsAre("void* var = goid();"));

  var.set_builtin(BPFHelper::TGID);

  ASSERT_OK_AND_THAT(GenScalarVariable(var),
                     ElementsAre("void* var = bpf_get_current_pid_tgid() >> 32;"));

  var.set_builtin(BPFHelper::TGID_PID);

  ASSERT_OK_AND_THAT(GenScalarVariable(var),
                     ElementsAre("void* var = bpf_get_current_pid_tgid();"));
}

TEST(GenStructVariableTest, Variables) {
  Struct st;

  st.set_name("socket_data_event_t");

  Struct::Field* field = nullptr;

  field = st.add_fields();
  field->set_name("i32");
  field->mutable_type()->set_scalar(ScalarType::INT32);

  field = st.add_fields();
  field->set_name("i64");
  field->mutable_type()->set_scalar(ScalarType::INT64);

  StructVariable st_var;

  st_var.set_name("st_var");
  st_var.set_struct_name("socket_data_event_t");

  auto* var_name = st_var.add_variable_names();
  var_name->set_name("foo");

  var_name = st_var.add_variable_names();
  var_name->set_name("bar");

  ASSERT_OK_AND_THAT(GenStructVariable(st, st_var),
                     ElementsAre("struct socket_data_event_t st_var = {};", "st_var.i32 = foo;",
                                 "st_var.i64 = bar;"));
}

TEST(GenMapStashActionTest, StashMap) {
  MapStashAction action;

  action.set_map_name("test");
  action.set_key_variable_name("foo");
  action.set_value_variable_name("bar");

  EXPECT_THAT(GenMapStashAction(action), ElementsAre("test.update(&foo, &bar);"));
}

TEST(GenOutputActionTest, Variables) {
  OutputAction action;

  action.set_perf_buffer_name("test");
  action.set_variable_name("foo");

  EXPECT_THAT(GenOutputAction(action), ElementsAre("test.perf_submit(ctx, &foo, sizeof(foo));"));
}

TEST(GenPhysicalProbeTest, EntryProbe) {
  PhysicalProbe probe;

  probe.set_name("syscall__probe_connect");

  Struct* st = probe.add_structs();
  st->set_name("socket_data_event_t");

  Struct::Field* field = st->add_fields();
  field->set_name("i32");
  field->mutable_type()->set_scalar(ScalarType::INT32);

  ScalarVariable* var = nullptr;

  var = probe.add_vars();
  var->set_name("key");
  var->set_val_type(ScalarType::UINT32);
  var->set_builtin(BPFHelper::TGID);

  var = probe.add_vars();
  var->set_name("var");
  var->set_val_type(ScalarType::INT32);
  var->set_reg(Register::SP);

  StructVariable* st_var = probe.add_st_vars();

  st_var->set_name("st_var");
  st_var->set_struct_name("socket_data_event_t");
  st_var->add_variable_names()->set_name("var");

  MapStashAction* map_stash_action = probe.add_map_stash_actions();

  map_stash_action->set_map_name("test");
  map_stash_action->set_key_variable_name("key");
  map_stash_action->set_value_variable_name("var");

  OutputAction* output_action = probe.add_output_actions();

  output_action->set_perf_buffer_name("data_events");
  output_action->set_variable_name("st_var");

  std::vector<std::string> expected = {"struct socket_data_event_t {",
                                       "  int32_t i32;",
                                       "};",
                                       "int syscall__probe_connect(struct pt_regs* ctx) {",
                                       "uint32_t key = bpf_get_current_pid_tgid() >> 32;",
                                       "int32_t var = PT_REGS_SP(ctx);",
                                       "struct socket_data_event_t st_var = {};",
                                       "st_var.i32 = var;",
                                       "test.update(&key, &var);",
                                       "data_events.perf_submit(ctx, &st_var, sizeof(st_var));",
                                       "return 0;",
                                       "}"};

  ASSERT_OK_AND_THAT(GenPhysicalProbe(probe), ElementsAreArray(expected));
}

}  // namespace dynamic_tracing
}  // namespace stirling
}  // namespace pl