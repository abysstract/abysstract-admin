import { useEffect, useState } from "react";
import { Outlet, Link, useLocation, useNavigate } from "react-router-dom";
import { ProLayout } from "@ant-design/pro-components";
import { Dropdown } from "antd";
import { UserOutlined, LogoutOutlined, KeyOutlined } from "@ant-design/icons";
import { useAccess } from "../runtime/access";
import { useAdminHost, useAdminRuntime, useModuleRegistry } from "../runtime/context";
import { buildLayoutRoutes } from "../menu";
import ChangePasswordModal from "../components/ChangePasswordModal";

export default function MainLayout() {
  const location = useLocation();
  const navigate = useNavigate();
  const host = useAdminHost();
  const { auth } = useAdminRuntime();
  const registry = useModuleRegistry();
  const { nickname, menus, pwdResetRequired } = useAccess();
  const [pwdModalOpen, setPwdModalOpen] = useState(false);
  // forced：pwdResetRequired 时首次弹出且不可关闭；改密成功后清除标志。
  const [pwdForced, setPwdForced] = useState(false);

  // 侧边菜单完全由后端返回的菜单树驱动（动态菜单）。
  const route = {
    path: "/",
    routes: buildLayoutRoutes(menus, registry),
  };

  // 首次登录/被重置：access 加载完成后自动弹出强制改密。
  // 仅触发一次（pwdResetRequired 由 access 拉取，clearPwdResetRequired 改密成功后调用）。
  useEffect(() => {
    if (pwdResetRequired && !pwdModalOpen && !pwdForced) {
      setPwdForced(true);
      setPwdModalOpen(true);
    }
  }, [pwdResetRequired, pwdModalOpen, pwdForced]);

  const openManualChangePwd = () => {
    setPwdForced(false);
    setPwdModalOpen(true);
  };

  const logout = () => {
    void auth.signOut().finally(() => {
      navigate("/login", { replace: true });
    });
  };

  return (
    <ProLayout
      title={host.title}
      logo={false}
      layout="mix"
      fixSiderbar
      location={{ pathname: location.pathname }}
      route={route}
      menuItemRender={(item, dom) => <Link to={item.path || "/"}>{dom}</Link>}
      avatarProps={{
        icon: <UserOutlined />,
        title: nickname,
        render: (_, dom) => (
          <Dropdown
            menu={{
              items: [
                {
                  key: "changePwd",
                  icon: <KeyOutlined />,
                  label: "修改密码",
                  onClick: openManualChangePwd,
                },
                { type: "divider" },
                { key: "logout", icon: <LogoutOutlined />, label: "退出登录", onClick: logout },
              ],
            }}
          >
            {dom}
          </Dropdown>
        ),
      }}
    >
      <Outlet />
      <ChangePasswordModal
        open={pwdModalOpen}
        forced={pwdForced}
        onClose={() => setPwdModalOpen(false)}
        onSuccess={() => {
          void auth.signOut().finally(() => {
            navigate("/login", { replace: true });
          });
        }}
      />
    </ProLayout>
  );
}
