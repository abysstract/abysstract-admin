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
      title="无可用菜单"
      subTitle="当前账号没有任何可访问的菜单或页面，请联系管理员分配角色与权限。"
      extra={
        <Button type="primary" onClick={relogin}>
          重新登录
        </Button>
      }
    />
  );
}
