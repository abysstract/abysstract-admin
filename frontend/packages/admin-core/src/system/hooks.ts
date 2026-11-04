import { useAdminRuntimeState } from "../runtime/context";
import type { SystemClients } from "./clients";
import type { SystemOperations } from "./operations";

export function useSystemClients(): SystemClients {
  return useAdminRuntimeState().clients;
}

export function useSystemOperations(): SystemOperations {
  return useAdminRuntimeState().operations;
}
