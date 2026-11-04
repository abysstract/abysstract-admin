export { createAdminApp, bindAdminAppPageLifecycle } from "./runtime/application";
export type {
  BindAdminAppPageLifecycleInput,
  AdminPageLifecycleTarget,
} from "./runtime/application";
export { createLocalAuthStorage, createMemoryAuthStorage } from "./runtime/storage";
export { useAdminRuntime } from "./runtime/context";
export { useAccess } from "./runtime/access";
export { useAppMessage, formatErrorMessage } from "./runtime/message";
export type {
  AdminApplication,
  AdminFrontendModule,
  AdminHost,
  AuthStorage,
} from "./runtime/types";
