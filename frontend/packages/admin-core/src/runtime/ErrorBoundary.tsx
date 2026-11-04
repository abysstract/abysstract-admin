import { Component, type ErrorInfo, type ReactNode } from "react";
import { Button, Result } from "antd";

type AppErrorBoundaryProps = {
  children: ReactNode;
  onError?: (error: unknown, info: ErrorInfo) => void;
};

// 渲染期异常的顶层兜底：替代整页白屏，展示可操作的错误页。
export class AppErrorBoundary extends Component<AppErrorBoundaryProps, { error: Error | null }> {
  state = { error: null as Error | null };

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    this.props.onError?.(error, info);
  }

  render() {
    if (this.state.error) {
      return (
        <Result
          status="error"
          title="页面出错了"
          subTitle="页面发生异常，请刷新页面重试；若反复出现，请联系管理员。"
          extra={
            <Button type="primary" onClick={() => window.location.reload()}>
              刷新页面
            </Button>
          }
        />
      );
    }
    return this.props.children;
  }
}
