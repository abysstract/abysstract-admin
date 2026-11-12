import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

// Real antd: the search bar's colours come from theme tokens and the button
// spacing from ConfigProvider, both of which the lightweight mock skips.
vi.unmock("antd");
vi.unmock("@ant-design/icons");

import { Button, ConfigProvider } from "antd";
import { ResourceSearchBar } from "./ResourceSearchBar";
import { AdminButtonSpacing } from "../AdminButtonSpacing";

describe("ResourceSearchBar", () => {
  it("reports keyword edits, searches on click and Enter, and reloads", () => {
    const onKeywordChange = vi.fn();
    const onSearch = vi.fn();
    const onReload = vi.fn();
    render(
      <ResourceSearchBar
        keyword="abc"
        onKeywordChange={onKeywordChange}
        onSearch={onSearch}
        onReload={onReload}
      />,
    );

    const input = screen.getByRole("textbox", { name: "关键词" });
    fireEvent.change(input, { target: { value: "abcd" } });
    expect(onKeywordChange).toHaveBeenLastCalledWith("abcd");

    fireEvent.keyDown(input, { key: "Enter", code: "Enter", keyCode: 13 });
    fireEvent.click(screen.getByRole("button", { name: /查\s?询/ }));
    expect(onSearch).toHaveBeenCalledTimes(2);

    fireEvent.click(screen.getByRole("button", { name: /刷\s?新/ }));
    expect(onReload).toHaveBeenCalledTimes(1);
  });

  it("hides the reload button without onReload and renders extra filters", () => {
    render(
      <ResourceSearchBar keyword="" onKeywordChange={() => {}} onSearch={() => {}}>
        <span>extra filter</span>
      </ResourceSearchBar>,
    );
    expect(screen.queryByRole("button", { name: /刷\s?新/ })).not.toBeInTheDocument();
    expect(screen.getByText("extra filter")).toBeInTheDocument();
  });

  it("takes its surface colours from the antd theme instead of fixed values", () => {
    const { container } = render(
      <ConfigProvider
        theme={{ token: { colorBgContainer: "#123456", colorBorderSecondary: "#654321" } }}
      >
        <ResourceSearchBar keyword="" onKeywordChange={() => {}} onSearch={() => {}} />
      </ConfigProvider>,
    );
    const surface = container.querySelector<HTMLElement>(".resource-table__search");
    expect(surface?.style.background).toBe("rgb(18, 52, 86)");
    expect(surface?.style.border).toContain("rgb(101, 67, 33)");
  });
});

describe("AdminButtonSpacing", () => {
  it("defaults to no space between two Chinese characters", () => {
    render(
      <AdminButtonSpacing>
        <Button>查询</Button>
      </AdminButtonSpacing>,
    );
    expect(screen.getByRole("button").textContent).toBe("查询");
  });

  it("keeps an explicit ConfigProvider choice from the host", () => {
    render(
      <ConfigProvider button={{ autoInsertSpace: true }}>
        <AdminButtonSpacing>
          <Button>查询</Button>
        </AdminButtonSpacing>
      </ConfigProvider>,
    );
    expect(screen.getByRole("button").textContent).toBe("查 询");
  });
});
