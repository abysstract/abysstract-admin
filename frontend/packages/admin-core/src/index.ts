export { createAdminApp, bindAdminAppPageLifecycle } from "./runtime/application";
export type {
  BindAdminAppPageLifecycleInput,
  AdminPageLifecycleTarget,
} from "./runtime/application";
export { createLocalAuthStorage, createMemoryAuthStorage } from "./runtime/storage";
export { useAdminRuntime } from "./runtime/context";
export { useAccess } from "./runtime/access";
export { useAppMessage, formatErrorMessage } from "./runtime/message";
export { defaultAdminTexts, useAdminTexts } from "./runtime/texts";
export type { AdminTexts, AdminTextsOverride } from "./runtime/texts";
export type {
  AdminApplication,
  AdminFrontendModule,
  AdminHost,
  AuthStorage,
} from "./runtime/types";
