import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import type { Transport } from "@connectrpc/connect";
import type { AdminHost, AdminRuntimeValue, SessionSnapshot } from "./types";
import type { SessionController } from "./session";
import type { SystemClients } from "../system/clients";
import type { SystemOperations } from "../system/operations";
import type { ModuleRegistry } from "../module/registry";

export interface AdminRuntimeState extends AdminRuntimeValue {
  readonly host: AdminHost;
  readonly session: SessionController;
  readonly clients: SystemClients;
  readonly operations: SystemOperations;
  readonly registry: ModuleRegistry;
}

const RuntimeContext = createContext<AdminRuntimeState | null>(null);

export function AdminRuntimeProvider({
  value,
  children,
}: {
  value: AdminRuntimeState;
  children: ReactNode;
}) {
  return <RuntimeContext.Provider value={value}>{children}</RuntimeContext.Provider>;
}

export function useAdminRuntimeState(): AdminRuntimeState {
  const value = useContext(RuntimeContext);
  if (!value) {
    throw new Error("useAdminRuntime 必须在 createAdminApp Root 内调用");
  }
  return value;
}

export function useAdminRuntime(): { transport: Transport; auth: { signOut(): Promise<void> } } {
  const value = useAdminRuntimeState();
  return { transport: value.transport, auth: value.auth };
}

export function useAdminHost(): AdminHost {
  return useAdminRuntimeState().host;
}

export function useSessionSnapshot(): SessionSnapshot {
  const { session } = useAdminRuntimeState();
  const [snapshot, setSnapshot] = useState(() => session.snapshot());
  useEffect(() => {
    setSnapshot(session.snapshot());
    return session.subscribe(() => {
      setSnapshot(session.snapshot());
    });
  }, [session]);
  return snapshot;
}

export function useModuleRegistry(): ModuleRegistry {
  return useAdminRuntimeState().registry;
}
