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

#include "src/carnot/planner/logical_planner.h"

#include <utility>

#include "src/shared/scriptspb/scripts.pb.h"

namespace px {
namespace carnot {
namespace planner {

using table_store::schemapb::Schema;

StatusOr<std::unique_ptr<RelationMap>> MakeRelationMapFromSchema(const Schema& schema_pb) {
  auto rel_map = std::make_unique<RelationMap>();
  for (auto& relation_pair : schema_pb.relation_map()) {
    px::table_store::schema::Relation rel;
    PL_RETURN_IF_ERROR(rel.FromProto(&relation_pair.second));
    rel_map->emplace(relation_pair.first, rel);
  }

  return rel_map;
}
StatusOr<std::unique_ptr<RelationMap>> MakeRelationMapFromDistributedState(
    const distributedpb::DistributedState& state_pb) {
  auto rel_map = std::make_unique<RelationMap>();
  for (const auto& schema_info : state_pb.schema_info()) {
    px::table_store::schema::Relation rel;
    PL_RETURN_IF_ERROR(rel.FromProto(&schema_info.relation()));
    rel_map->emplace(schema_info.name(), rel);
  }

  return rel_map;
}

StatusOr<std::unique_ptr<CompilerState>> CreateCompilerState(
    const distributedpb::LogicalPlannerState& logical_state, RegistryInfo* registry_info,
    int64_t max_output_rows_per_table) {
  PL_ASSIGN_OR_RETURN(std::unique_ptr<RelationMap> rel_map,
                      MakeRelationMapFromDistributedState(logical_state.distributed_state()));
  // Create a CompilerState obj using the relation map and grabbing the current time.

  return std::make_unique<planner::CompilerState>(
      std::move(rel_map), registry_info, px::CurrentTimeNS(), max_output_rows_per_table,
      logical_state.result_address(), logical_state.result_ssl_targetname());
}

StatusOr<std::unique_ptr<LogicalPlanner>> LogicalPlanner::Create(const udfspb::UDFInfo& udf_info) {
  auto planner = std::unique_ptr<LogicalPlanner>(new LogicalPlanner());
  PL_RETURN_IF_ERROR(planner->Init(udf_info));
  return planner;
}

Status LogicalPlanner::Init(const udfspb::UDFInfo& udf_info) {
  compiler_ = compiler::Compiler();
  registry_info_ = std::make_unique<planner::RegistryInfo>();
  PL_RETURN_IF_ERROR(registry_info_->Init(udf_info));

  PL_ASSIGN_OR_RETURN(distributed_planner_, distributed::DistributedPlanner::Create());
  return Status::OK();
}

StatusOr<std::unique_ptr<distributed::DistributedPlan>> LogicalPlanner::Plan(
    const distributedpb::LogicalPlannerState& logical_state,
    const plannerpb::QueryRequest& query_request) {
  // Compile into the IR.
  auto ms = logical_state.plan_options().max_output_rows_per_table();
  VLOG(1) << "Max output rows: " << ms;
  PL_ASSIGN_OR_RETURN(std::unique_ptr<CompilerState> compiler_state,
                      CreateCompilerState(logical_state, registry_info_.get(), ms));

  std::vector<plannerpb::FuncToExecute> exec_funcs(query_request.exec_funcs().begin(),
                                                   query_request.exec_funcs().end());
  PL_ASSIGN_OR_RETURN(
      std::shared_ptr<IR> single_node_plan,
      compiler_.CompileToIR(query_request.query_str(), compiler_state.get(), exec_funcs));
  // Create the distributed plan.
  return distributed_planner_->Plan(logical_state.distributed_state(), compiler_state.get(),
                                    single_node_plan.get());
}

StatusOr<std::unique_ptr<compiler::MutationsIR>> LogicalPlanner::CompileTrace(
    const distributedpb::LogicalPlannerState& logical_state,
    const plannerpb::CompileMutationsRequest& mutations_req) {
  // Compile into the IR.
  auto ms = logical_state.plan_options().max_output_rows_per_table();
  VLOG(1) << "Max output rows: " << ms;
  PL_ASSIGN_OR_RETURN(std::unique_ptr<CompilerState> compiler_state,
                      CreateCompilerState(logical_state, registry_info_.get(), ms));

  std::vector<plannerpb::FuncToExecute> exec_funcs(mutations_req.exec_funcs().begin(),
                                                   mutations_req.exec_funcs().end());

  return compiler_.CompileTrace(mutations_req.query_str(), compiler_state.get(), exec_funcs);
}

StatusOr<shared::scriptspb::FuncArgsSpec> LogicalPlanner::GetMainFuncArgsSpec(
    const plannerpb::QueryRequest& query_request) {
  PL_ASSIGN_OR_RETURN(std::unique_ptr<CompilerState> compiler_state,
                      CreateCompilerState({}, registry_info_.get(), 0));

  return compiler_.GetMainFuncArgsSpec(query_request.query_str(), compiler_state.get());
}

StatusOr<px::shared::scriptspb::VisFuncsInfo> LogicalPlanner::GetVisFuncsInfo(
    const std::string& script_str) {
  PL_ASSIGN_OR_RETURN(std::unique_ptr<CompilerState> compiler_state,
                      CreateCompilerState({}, registry_info_.get(), 0));

  return compiler_.GetVisFuncsInfo(script_str, compiler_state.get());
}

}  // namespace planner
}  // namespace carnot
}  // namespace px
