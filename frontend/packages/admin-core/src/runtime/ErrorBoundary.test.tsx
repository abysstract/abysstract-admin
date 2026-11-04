import { render, screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { describe, expect, it, vi } from "vitest";
import { AppErrorBoundary } from "./ErrorBoundary";

function Bomb({ message }: { message: string }): ReactElement {
  throw new Error(message);
}

describe("AppErrorBoundary", () => {
  it("renders the fallback page instead of crashing and reports the error", () => {
    const onError = vi.fn();
    const { container } = render(
      <AppErrorBoundary onError={onError}>
        <Bomb message="渲染炸了" />
      </AppErrorBoundary>,
    );
    expect(screen.getByText("页面出错了")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "刷新页面" })).toBeInTheDocument();
    expect(container.textContent).not.toContain("渲染炸了");
    expect(onError).toHaveBeenCalledTimes(1);
    expect(onError.mock.calls[0][0]).toBeInstanceOf(Error);
  });

  it("renders children when no error is thrown", () => {
    render(
      <AppErrorBoundary>
        <div>正常内容</div>
      </AppErrorBoundary>,
    );
    expect(screen.getByText("正常内容")).toBeInTheDocument();
  });
});
