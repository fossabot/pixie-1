#pragma once

#include "src/common/base/base.h"
#include "src/stirling/dynamic_tracing/ir/logical.pb.h"
#include "src/stirling/dynamic_tracing/ir/physical.pb.h"

namespace pl {
namespace stirling {
namespace dynamic_tracing {

/**
 * Transforms any logical probes inside a program into entry and return probes.
 * Also automatically adds any required supporting maps and implicit outputs.
 */
StatusOr<ir::logical::Program> TransformLogicalProgram(const ir::logical::Program& input_program);

}  // namespace dynamic_tracing
}  // namespace stirling
}  // namespace pl