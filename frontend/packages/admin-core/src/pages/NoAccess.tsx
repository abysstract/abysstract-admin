import { Button, Result } from "antd";
import { useNavigate } from "react-router-dom";
import { useAdminRuntime } from "../runtime/context";

// 当前账号没有任何可访问的菜单/页面时的兜底页，避免重定向到自身造成空白。
export default function NoAccess() {
  const navigate = useNavigate();
  const { auth } = useAdminRuntime();

  const relogin = () => {
    void auth.signOut().finally(() => {
      navigate("/login", { replace: true });
    });
  };

  return (
    <Result
      status="403"
      title="没有可访问的页面"
      subTitle="这个账号还没有分配角色或菜单权限。管理员分配后，重新登录即可"
      extra={
        <Button type="primary" onClick={relogin}>
          重新登录
        </Button>
      }
    />
  );
}
