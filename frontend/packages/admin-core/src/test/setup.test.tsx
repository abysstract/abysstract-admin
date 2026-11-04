import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { Switch, Upload } from "antd";
import { describe, expect, it, vi } from "vitest";

describe("全局 antd 轻量 mock", () => {
  it("Switch 保留 checked 状态并向 onChange 传递切换后的值", () => {
    const onChange = vi.fn();
    render(<Switch checked onChange={onChange} />);

    const input = screen.getByRole("checkbox");
    expect(input).toBeChecked();

    fireEvent.click(input);
    expect(onChange).toHaveBeenCalledWith(false, expect.anything());
  });

  it("Upload 调用 beforeUpload，并在返回 false 时保留 onChange 而跳过 customRequest", async () => {
    const beforeUpload = vi.fn(() => false);
    const onChange = vi.fn();
    const customRequest = vi.fn();
    const { container } = render(
      <Upload beforeUpload={beforeUpload} onChange={onChange} customRequest={customRequest}>
        <button type="button">导入</button>
      </Upload>,
    );
    const input = container.querySelector('input[type="file"]');
    expect(input).not.toBeNull();

    const file = new File(["sheet"], "users.xlsx", {
      type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    });
    fireEvent.change(input!, { target: { files: [file] } });

    await waitFor(() => expect(beforeUpload).toHaveBeenCalledWith(file, [file]));
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ file, fileList: [file] }));
    expect(customRequest).not.toHaveBeenCalled();
  });

  it("Upload 在 LIST_IGNORE 时不触发 onChange 或 customRequest", async () => {
    const beforeUpload = vi.fn(() => Upload.LIST_IGNORE);
    const onChange = vi.fn();
    const customRequest = vi.fn();
    const { container } = render(
      <Upload beforeUpload={beforeUpload} onChange={onChange} customRequest={customRequest}>
        <button type="button">更换头像</button>
      </Upload>,
    );
    const input = container.querySelector('input[type="file"]');
    expect(input).not.toBeNull();

    fireEvent.change(input!, { target: { files: [new File(["large"], "large.png")] } });

    await waitFor(() => expect(beforeUpload).toHaveBeenCalledTimes(1));
    expect(onChange).not.toHaveBeenCalled();
    expect(customRequest).not.toHaveBeenCalled();
  });

  it("Upload 在 beforeUpload 允许时调用 customRequest", async () => {
    const customRequest = vi.fn();
    const { container } = render(
      <Upload beforeUpload={() => true} customRequest={customRequest}>
        <button type="button">更换头像</button>
      </Upload>,
    );
    const input = container.querySelector('input[type="file"]');
    expect(input).not.toBeNull();
    const file = new File(["avatar"], "avatar.png", { type: "image/png" });

    fireEvent.change(input!, { target: { files: [file] } });

    await waitFor(() =>
      expect(customRequest).toHaveBeenCalledWith(expect.objectContaining({ file })),
    );
  });
});
