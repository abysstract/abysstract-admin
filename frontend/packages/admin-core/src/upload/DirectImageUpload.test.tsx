import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { DirectImageUpload } from "./DirectImageUpload";

const upload = vi.fn();
const messageError = vi.fn();

vi.mock("./useDirectUpload", () => ({
  useDirectUpload: () => ({
    upload,
    resolveUrl: (url: string) => (url.startsWith("/") ? `http://api.test${url}` : url),
  }),
}));

vi.mock("../runtime/message", () => ({
  useAppMessage: () => ({ error: messageError }),
  formatErrorMessage: (e: unknown, fallback: string) => (e instanceof Error ? e.message : fallback),
}));

function selectFile(container: HTMLElement, file: File) {
  const input = container.querySelector("input[type=file]") as HTMLInputElement;
  fireEvent.change(input, { target: { files: [file] } });
}

describe("DirectImageUpload", () => {
  beforeEach(() => {
    upload.mockReset();
    messageError.mockReset();
  });

  it("uploads directly and reports the object key as the form value", async () => {
    upload.mockResolvedValue({
      key: "files/a.png",
      url: "http://api.test/static/files/a.png",
      size: 3,
    });
    const onChange = vi.fn();
    const onUploaded = vi.fn();
    const { container } = render(<DirectImageUpload onChange={onChange} onUploaded={onUploaded} />);

    const file = new File(["png"], "a.png", { type: "image/png" });
    selectFile(container, file);

    await waitFor(() => expect(onChange).toHaveBeenCalledWith("files/a.png"));
    expect(upload).toHaveBeenCalledWith(
      file,
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
    expect(onUploaded).toHaveBeenCalledWith(expect.objectContaining({ key: "files/a.png" }));
  });

  it("previews a saved key through resolveUrl", () => {
    const { container } = render(
      <DirectImageUpload value="files/b.png" resolveUrl={(key) => `/static/${key}`} />,
    );
    expect(container.querySelector("img")?.getAttribute("src")).toBe(
      "http://api.test/static/files/b.png",
    );
  });

  it("rejects disallowed types and oversized files before uploading", async () => {
    const { container } = render(<DirectImageUpload maxBytes={2} />);
    selectFile(container, new File(["x"], "a.svg", { type: "image/svg+xml" }));
    selectFile(container, new File(["xyz"], "a.png", { type: "image/png" }));
    await waitFor(() => expect(messageError).toHaveBeenCalledTimes(2));
    expect(upload).not.toHaveBeenCalled();
  });

  it("shows upload errors and keeps the value", async () => {
    upload.mockRejectedValue(new Error("上传失败（HTTP 403）"));
    const onChange = vi.fn();
    const { container } = render(<DirectImageUpload onChange={onChange} />);
    selectFile(container, new File(["png"], "a.png", { type: "image/png" }));
    await waitFor(() => expect(messageError).toHaveBeenCalledWith("上传失败（HTTP 403）"));
    expect(onChange).not.toHaveBeenCalled();
  });

  it("clears the value with the remove button", () => {
    const onChange = vi.fn();
    render(<DirectImageUpload value="files/b.png" onChange={onChange} />);
    fireEvent.click(screen.getByLabelText("移除图片"));
    expect(onChange).toHaveBeenCalledWith(undefined);
  });
});
