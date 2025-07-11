// @generated by protoc-gen-es v2.2.3 with parameter "target=js+dts,import_extension=none,json_types=true"
// @generated from file cassie/eval.proto (syntax proto3)
/* eslint-disable */

import { enumDesc, fileDesc, messageDesc, tsEnum } from "@bufbuild/protobuf/codegenv1";
import { file_buf_validate_validate } from "../buf/validate/validate_pb";

/**
 * Describes the file cassie/eval.proto.
 */
export const file_cassie_eval = /*@__PURE__*/
  fileDesc("ChFjYXNzaWUvZXZhbC5wcm90byKhBwoJQXNzZXJ0aW9uEhgKBG5hbWUYASABKAlCCrpIB8gBAXICEAESJQoEdHlwZRgCIAEoDjIPLkFzc2VydGlvbi5UeXBlQga6SAPIAQESIQoGcmVzdWx0GAMgASgOMhEuQXNzZXJ0aW9uLlJlc3VsdBI7ChNzaGVsbF9yZXF1aXJlZF9mbGFnGAQgASgLMhwuQXNzZXJ0aW9uLlNoZWxsUmVxdWlyZWRGbGFnSAASNAoPdG9vbF9pbnZvY2F0aW9uGAUgASgLMhkuQXNzZXJ0aW9uLlRvb2xJbnZvY2F0aW9uSAASMgoOZmlsZV9yZXRyaWV2YWwYBiABKAsyGC5Bc3NlcnRpb24uRmlsZVJldHJpZXZhbEgAEigKCWxsbV9qdWRnZRgHIAEoCzITLkFzc2VydGlvbi5MTE1KdWRnZUgAEjQKD2NvZGVibG9ja19yZWdleBgIIAEoCzIZLkFzc2VydGlvbi5Db2RlYmxvY2tSZWdleEgAEhYKDmZhaWx1cmVfcmVhc29uGAkgASgJGkwKEVNoZWxsUmVxdWlyZWRGbGFnEhsKB2NvbW1hbmQYASABKAlCCrpIB8gBAXICEAESGgoFZmxhZ3MYAiADKAlCC7pICMgBAZIBAggBGi8KDlRvb2xJbnZvY2F0aW9uEh0KCXRvb2xfbmFtZRgBIAEoCUIKukgHyAEBcgIQARo/Cg1GaWxlUmV0cmlldmFsEhsKB2ZpbGVfaWQYASABKAlCCrpIB8gBAXICEAESEQoJZmlsZV9uYW1lGAIgASgJGiYKCExMTUp1ZGdlEhoKBnByb21wdBgBIAEoCUIKukgHyAEBcgIQARorCg5Db2RlYmxvY2tSZWdleBIZCgVyZWdleBgBIAEoCUIKukgHyAEBcgIQASKUAQoEVHlwZRIQCgxUWVBFX1VOS05PV04QABIcChhUWVBFX1NIRUxMX1JFUVVJUkVEX0ZMQUcQARIVChFUWVBFX1RPT0xfSU5WT0tFRBACEhcKE1RZUEVfRklMRV9SRVRSSUVWRUQQAxISCg5UWVBFX0xMTV9KVURHRRAEEhgKFFRZUEVfQ09ERUJMT0NLX1JFR0VYEAUiUwoGUmVzdWx0EhIKDlJFU1VMVF9VTktOT1dOEAASDwoLUkVTVUxUX1RSVUUQARIQCgxSRVNVTFRfRkFMU0UQAhISCg5SRVNVTFRfU0tJUFBFRBADQhAKB3BheWxvYWQSBbpIAggBIpoBCgpFdmFsU2FtcGxlEhgKBGtpbmQYASABKAlCCrpIB8gBAXICEAESJQoIbWV0YWRhdGEYAiABKAsyCy5PYmplY3RNZXRhQga6SAPIAQESHgoKaW5wdXRfdGV4dBgDIAEoCUIKukgHyAEBcgIQARIrCgphc3NlcnRpb25zGAQgAygLMgouQXNzZXJ0aW9uQgu6SAjIAQGSAQIIASIrCgtFdmFsRGF0YXNldBIcCgdzYW1wbGVzGAEgAygLMgsuRXZhbFNhbXBsZSImCgpPYmplY3RNZXRhEhgKBG5hbWUYASABKAlCCrpIB8gBAXICEAEiegoORXhwZXJpbWVudFNwZWMSIAoMZGF0YXNldF9wYXRoGAEgASgJQgq6SAfIAQFyAhABEh4KCm91dHB1dF9kaXIYAiABKAlCCrpIB8gBAXICEAESJgoSaW5mZXJlbmNlX2VuZHBvaW50GAMgASgJQgq6SAfIAQFyAhABIpUBCgpFeHBlcmltZW50Eh8KC2FwaV92ZXJzaW9uGAEgASgJQgq6SAfIAQFyAhABEhgKBGtpbmQYAiABKAlCCrpIB8gBAXICEAESJQoIbWV0YWRhdGEYAyABKAsyCy5PYmplY3RNZXRhQga6SAPIAQESJQoEc3BlYxgEIAEoCzIPLkV4cGVyaW1lbnRTcGVjQga6SAPIAQFCQUIJRXZhbFByb3RvUAFaMmdpdGh1Yi5jb20vamxld2kvY2xvdWQtYXNzaXN0YW50L3Byb3Rvcy9nZW4vY2Fzc2llYgZwcm90bzM", [file_buf_validate_validate]);

/**
 * Describes the message Assertion.
 * Use `create(AssertionSchema)` to create a new message.
 */
export const AssertionSchema = /*@__PURE__*/
  messageDesc(file_cassie_eval, 0);

/**
 * Describes the message Assertion.ShellRequiredFlag.
 * Use `create(Assertion_ShellRequiredFlagSchema)` to create a new message.
 */
export const Assertion_ShellRequiredFlagSchema = /*@__PURE__*/
  messageDesc(file_cassie_eval, 0, 0);

/**
 * Describes the message Assertion.ToolInvocation.
 * Use `create(Assertion_ToolInvocationSchema)` to create a new message.
 */
export const Assertion_ToolInvocationSchema = /*@__PURE__*/
  messageDesc(file_cassie_eval, 0, 1);

/**
 * Describes the message Assertion.FileRetrieval.
 * Use `create(Assertion_FileRetrievalSchema)` to create a new message.
 */
export const Assertion_FileRetrievalSchema = /*@__PURE__*/
  messageDesc(file_cassie_eval, 0, 2);

/**
 * Describes the message Assertion.LLMJudge.
 * Use `create(Assertion_LLMJudgeSchema)` to create a new message.
 */
export const Assertion_LLMJudgeSchema = /*@__PURE__*/
  messageDesc(file_cassie_eval, 0, 3);

/**
 * Describes the message Assertion.CodeblockRegex.
 * Use `create(Assertion_CodeblockRegexSchema)` to create a new message.
 */
export const Assertion_CodeblockRegexSchema = /*@__PURE__*/
  messageDesc(file_cassie_eval, 0, 4);

/**
 * Describes the enum Assertion.Type.
 */
export const Assertion_TypeSchema = /*@__PURE__*/
  enumDesc(file_cassie_eval, 0, 0);

/**
 * What we are checking for.
 *
 * @generated from enum Assertion.Type
 */
export const Assertion_Type = /*@__PURE__*/
  tsEnum(Assertion_TypeSchema);

/**
 * Describes the enum Assertion.Result.
 */
export const Assertion_ResultSchema = /*@__PURE__*/
  enumDesc(file_cassie_eval, 0, 1);

/**
 * Outcome of an assertion after a test run.
 *
 * @generated from enum Assertion.Result
 */
export const Assertion_Result = /*@__PURE__*/
  tsEnum(Assertion_ResultSchema);

/**
 * Describes the message EvalSample.
 * Use `create(EvalSampleSchema)` to create a new message.
 */
export const EvalSampleSchema = /*@__PURE__*/
  messageDesc(file_cassie_eval, 1);

/**
 * Describes the message EvalDataset.
 * Use `create(EvalDatasetSchema)` to create a new message.
 */
export const EvalDatasetSchema = /*@__PURE__*/
  messageDesc(file_cassie_eval, 2);

/**
 * Describes the message ObjectMeta.
 * Use `create(ObjectMetaSchema)` to create a new message.
 */
export const ObjectMetaSchema = /*@__PURE__*/
  messageDesc(file_cassie_eval, 3);

/**
 * Describes the message ExperimentSpec.
 * Use `create(ExperimentSpecSchema)` to create a new message.
 */
export const ExperimentSpecSchema = /*@__PURE__*/
  messageDesc(file_cassie_eval, 4);

/**
 * Describes the message Experiment.
 * Use `create(ExperimentSchema)` to create a new message.
 */
export const ExperimentSchema = /*@__PURE__*/
  messageDesc(file_cassie_eval, 5);

