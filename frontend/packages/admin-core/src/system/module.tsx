import { lazy, type ReactNode } from "react";
import {
  ApartmentOutlined,
  BookOutlined,
  LoginOutlined,
  MenuOutlined,
  NotificationOutlined,
  ProfileOutlined,
  SettingOutlined,
  TeamOutlined,
  ToolOutlined,
  UserOutlined,
} from "@ant-design/icons";
import type { AdminFrontendModule } from "../runtime/types";

export const CORE_SYSTEM_PAGE_KEYS = [
  "system/user/index",
  "system/role/index",
  "system/menu/index",
  "system/dept/index",
  "system/dict/index",
  "system/config/index",
  "system/notice/index",
  "monitor/operlog/index",
  "monitor/logininfor/index",
  "dashboard/index",
  "profile/index",
] as const;

const pages = {
  "system/user/index": lazy(() => import("../pages/system/UserList")),
  "system/role/index": lazy(() => import("../pages/system/RoleList")),
  "system/menu/index": lazy(() => import("../pages/system/MenuList")),
  "system/dept/index": lazy(() => import("../pages/system/DeptList")),
  "system/dict/index": lazy(() => import("../pages/system/DictList")),
  "system/config/index": lazy(() => import("../pages/system/ConfigList")),
  "system/notice/index": lazy(() => import("../pages/system/NoticeList")),
  "monitor/operlog/index": lazy(() => import("../pages/system/OperLogList")),
  "monitor/logininfor/index": lazy(() => import("../pages/system/LoginLogList")),
  "dashboard/index": lazy(() => import("../pages/dashboard")),
  "profile/index": lazy(() => import("../pages/profile")),
} as const;

const icons: Readonly<Record<string, ReactNode>> = {
  setting: <SettingOutlined />,
  user: <UserOutlined />,
  team: <TeamOutlined />,
  menu: <MenuOutlined />,
  apartment: <ApartmentOutlined />,
  book: <BookOutlined />,
  tool: <ToolOutlined />,
  notification: <NotificationOutlined />,
  profile: <ProfileOutlined />,
  login: <LoginOutlined />,
};

export const coreSystemModule: AdminFrontendModule = Object.freeze({
  name: "system",
  pages: Object.freeze({ ...pages }),
  icons: Object.freeze({ ...icons }),
});
