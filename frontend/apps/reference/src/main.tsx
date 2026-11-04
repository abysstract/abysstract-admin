import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import zhCN from "antd/locale/zh_CN";
import {
  bindAdminAppPageLifecycle,
  createAdminApp,
  createLocalAuthStorage,
} from "@abysstract/admin-core";
import { modules } from "./modules";
import "antd/dist/reset.css";
import "@abysstract/admin-core/styles.css";

const mode = import.meta.env.PROD ? "production" : "development";
const configuredBase = (import.meta.env.VITE_API_BASE as string | undefined)?.trim() ?? "";
const apiBaseUrl =
  configuredBase || (mode === "development" ? "http://localhost:8080" : configuredBase);

const application = createAdminApp({
  host: {
    apiBaseUrl,
    mode,
    authStorage: createLocalAuthStorage("abysstract_admin_token"),
    title: "Abysstract 后台",
    locale: zhCN,
    ...(import.meta.env.DEV
      ? { developmentLogin: { username: "admin", password: "admin123" } }
      : {}),
  },
  modules,
});

const root = ReactDOM.createRoot(document.getElementById("root")!);
root.render(
  <React.StrictMode>
    <BrowserRouter>
      <application.Root />
    </BrowserRouter>
  </React.StrictMode>,
);

bindAdminAppPageLifecycle({ application, root });
