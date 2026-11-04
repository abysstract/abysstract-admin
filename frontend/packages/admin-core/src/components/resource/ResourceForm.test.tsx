import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { dictDataSchema } from "../../gen/resource/dict-data.schema.gen";
import { dictTypeSchema } from "../../gen/resource/dict-type.schema.gen";
import { ResourceForm, type ResourceFormSubmission } from "./ResourceForm";

describe("ResourceForm", () => {
  it("renders submit errors with the shared error presentation class", () => {
    render(
      <ResourceForm
        schema={dictTypeSchema}
        mode="create"
        error={new Error("提交失败")}
        onCreate={vi.fn()}
      />,
    );

    expect(screen.getByRole("alert")).toHaveClass("resource-form__error");
  });

  it("renders only ordinary updatable fields and reports remark-only touched/dirty state", () => {
    const onSubmit = vi.fn<(submission: ResourceFormSubmission) => void>();
    render(
      <ResourceForm
        schema={dictTypeSchema}
        mode="update"
        initialValues={{ id: 7n, dictName: "original", dictType: "locked", remark: "before" }}
        onSubmit={onSubmit}
      />,
    );

    expect(screen.getByLabelText("dictName")).toBeInTheDocument();
    expect(screen.getByLabelText("remark")).toBeInTheDocument();
    expect(screen.queryByLabelText("dictType")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("id")).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("remark"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    expect(onSubmit).toHaveBeenCalledOnce();
    const submission = onSubmit.mock.calls[0][0];
    expect(submission.values).toMatchObject({ id: 7n, dictName: "original", remark: "" });
    expect([...submission.touched]).toEqual(["remark"]);
    expect([...submission.dirty]).toEqual(["remark"]);
  });

  it("preserves an explicitly touched numeric zero", () => {
    const onSubmit = vi.fn<(submission: ResourceFormSubmission) => void>();
    render(
      <ResourceForm
        schema={dictDataSchema}
        mode="update"
        initialValues={{ id: 11n, dictSort: 9, dictLabel: "label", dictValue: "value", remark: "" }}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.change(screen.getByLabelText("dictSort"), { target: { value: "0" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    const submission = onSubmit.mock.calls[0][0];
    expect(submission.values.dictSort).toBe(0);
    expect([...submission.touched]).toEqual(["dictSort"]);
    expect([...submission.dirty]).toEqual(["dictSort"]);
  });
});

import type { RelationOptionLoader } from "./relation";

describe("ResourceForm relation loading", () => {
  it("keeps target errors visible instead of replacing options with an empty list", async () => {
    const denied = new Error("PermissionDenied");
    const loader: RelationOptionLoader<string> = {
      remoteSearchEnabled: true,
      load: vi.fn().mockRejectedValue(denied),
    };
    render(
      <ResourceForm
        schema={dictDataSchema}
        mode="create"
        relationLoaders={{ dictType: loader }}
        onSubmit={vi.fn()}
      />,
    );

    expect(await screen.findByRole("alert")).toHaveTextContent("PermissionDenied");
    expect(screen.getByLabelText("dictType search")).toBeInTheDocument();
  });

  it("supports page continuation without rendering remote search for non-searchable targets", async () => {
    const load = vi
      .fn()
      .mockResolvedValueOnce({ options: [{ value: "one", label: "One" }], nextPage: 2 })
      .mockResolvedValueOnce({ options: [{ value: "two", label: "Two" }] });
    const loader: RelationOptionLoader<string> = { remoteSearchEnabled: false, load };
    render(
      <ResourceForm
        schema={dictDataSchema}
        mode="create"
        relationLoaders={{ dictType: loader }}
        onSubmit={vi.fn()}
      />,
    );

    expect(await screen.findByText("One")).toBeInTheDocument();
    expect(screen.queryByLabelText("dictType search")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "加载更多" }));
    expect(await screen.findByText("Two")).toBeInTheDocument();
    expect(load).toHaveBeenLastCalledWith({ pageNum: 2, pageSize: 20 });
  });
});

it("retains the active remote-search keyword when loading the next page", async () => {
  const load = vi
    .fn()
    .mockResolvedValueOnce({ options: [] })
    .mockResolvedValueOnce({ options: [{ value: "one", label: "One" }], nextPage: 2 })
    .mockResolvedValueOnce({ options: [{ value: "two", label: "Two" }] });
  const loader: RelationOptionLoader<string> = { remoteSearchEnabled: true, load };
  render(
    <ResourceForm
      schema={dictDataSchema}
      mode="create"
      relationLoaders={{ dictType: loader }}
      onSubmit={vi.fn()}
    />,
  );

  fireEvent.change(screen.getByLabelText("dictType search"), { target: { value: "needle" } });
  expect(await screen.findByText("One")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "加载更多" }));
  expect(await screen.findByText("Two")).toBeInTheDocument();
  expect(load).toHaveBeenLastCalledWith({ pageNum: 2, pageSize: 20, keyword: "needle" });
});

it("resets values, touched, and dirty when initialValues changes without remounting", () => {
  const onSubmit = vi.fn<(submission: ResourceFormSubmission) => void>();
  const { rerender } = render(
    <ResourceForm
      schema={dictTypeSchema}
      mode="update"
      resetKey="dict-type:1"
      initialValues={{ id: 1n, dictName: "A", remark: "first" }}
      onSubmit={onSubmit}
    />,
  );
  fireEvent.change(screen.getByLabelText("remark"), { target: { value: "edited" } });

  rerender(
    <ResourceForm
      schema={dictTypeSchema}
      mode="update"
      resetKey="dict-type:2"
      initialValues={{ id: 2n, dictName: "B", remark: "second" }}
      onSubmit={onSubmit}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "提交" }));

  const submission = onSubmit.mock.calls[0][0];
  expect(submission.values).toMatchObject({ id: 2n, dictName: "B", remark: "second" });
  expect([...submission.touched]).toEqual([]);
  expect([...submission.dirty]).toEqual([]);
});

it("keeps the newest relation search results when an older request resolves last", async () => {
  type RelationResult = {
    options: ReadonlyArray<{ value: string; label: string }>;
    nextPage?: number;
  };
  const pending: Array<{ resolve: (result: RelationResult) => void }> = [];
  const load = vi.fn(
    () =>
      new Promise<RelationResult>((resolve) => {
        pending.push({ resolve });
      }),
  );
  const loader: RelationOptionLoader<string> = { remoteSearchEnabled: true, load };
  render(
    <ResourceForm
      schema={dictDataSchema}
      mode="create"
      relationLoaders={{ dictType: loader }}
      onSubmit={vi.fn()}
    />,
  );

  await waitFor(() => expect(load).toHaveBeenCalledTimes(1));
  fireEvent.change(screen.getByLabelText("dictType search"), { target: { value: "a" } });
  fireEvent.change(screen.getByLabelText("dictType search"), { target: { value: "ab" } });
  await waitFor(() => expect(load).toHaveBeenCalledTimes(3));

  await act(async () => {
    pending[2].resolve({ options: [{ value: "ab", label: "AB" }] });
  });
  expect(await screen.findByText("AB")).toBeInTheDocument();

  await act(async () => {
    pending[1].resolve({ options: [{ value: "a", label: "A" }] });
  });
  expect(screen.getByText("AB")).toBeInTheDocument();
  expect(screen.queryByText("A")).not.toBeInTheDocument();
});

it("adopts asynchronously loaded initialValues for the same resetKey", () => {
  const onSubmit = vi.fn<(submission: ResourceFormSubmission) => void>();
  const { rerender } = render(
    <ResourceForm
      schema={dictTypeSchema}
      mode="update"
      resetKey="dict-type:1"
      initialValues={{}}
      onSubmit={onSubmit}
    />,
  );

  rerender(
    <ResourceForm
      schema={dictTypeSchema}
      mode="update"
      resetKey="dict-type:1"
      initialValues={{ id: 1n, dictName: "loaded", remark: "from-api" }}
      onSubmit={onSubmit}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "提交" }));

  const submission = onSubmit.mock.calls[0][0];
  expect(submission.values).toMatchObject({ id: 1n, dictName: "loaded", remark: "from-api" });
  expect([...submission.touched]).toEqual([]);
  expect([...submission.dirty]).toEqual([]);
});

it("does not discard dirty local edits when the same resetKey receives refreshed initialValues", () => {
  const onSubmit = vi.fn<(submission: ResourceFormSubmission) => void>();
  const { rerender } = render(
    <ResourceForm
      schema={dictTypeSchema}
      mode="update"
      resetKey="dict-type:1"
      initialValues={{ id: 1n, dictName: "A", remark: "first" }}
      onSubmit={onSubmit}
    />,
  );
  fireEvent.change(screen.getByLabelText("remark"), { target: { value: "editing" } });

  rerender(
    <ResourceForm
      schema={dictTypeSchema}
      mode="update"
      resetKey="dict-type:1"
      initialValues={{ id: 1n, dictName: "A", remark: "server-refresh" }}
      onSubmit={onSubmit}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "提交" }));

  const submission = onSubmit.mock.calls[0][0];
  expect(submission.values).toMatchObject({ id: 1n, dictName: "A", remark: "editing" });
  expect([...submission.touched]).toEqual(["remark"]);
  expect([...submission.dirty]).toEqual(["remark"]);
});

it("keeps the original dirty baseline while editing after a same-key refresh", () => {
  const onSubmit = vi.fn<(submission: ResourceFormSubmission) => void>();
  const { rerender } = render(
    <ResourceForm
      schema={dictTypeSchema}
      mode="update"
      resetKey="dict-type:1"
      initialValues={{ id: 1n, dictName: "A", remark: "first" }}
      onSubmit={onSubmit}
    />,
  );
  fireEvent.change(screen.getByLabelText("remark"), { target: { value: "editing" } });

  rerender(
    <ResourceForm
      schema={dictTypeSchema}
      mode="update"
      resetKey="dict-type:1"
      initialValues={{ id: 1n, dictName: "A", remark: "server-refresh" }}
      onSubmit={onSubmit}
    />,
  );
  fireEvent.change(screen.getByLabelText("remark"), { target: { value: "first" } });
  fireEvent.click(screen.getByRole("button", { name: "提交" }));

  const submission = onSubmit.mock.calls[0][0];
  expect(submission.values).toMatchObject({ id: 1n, dictName: "A", remark: "first" });
  expect([...submission.touched]).toEqual(["remark"]);
  expect([...submission.dirty]).toEqual([]);
});

it("adopts same-key refreshed initialValues after the user reverts back to a clean state", () => {
  const onSubmit = vi.fn<(submission: ResourceFormSubmission) => void>();
  const { rerender } = render(
    <ResourceForm
      schema={dictTypeSchema}
      mode="update"
      resetKey="dict-type:1"
      initialValues={{ id: 1n, dictName: "A", remark: "first" }}
      onSubmit={onSubmit}
    />,
  );
  fireEvent.change(screen.getByLabelText("remark"), { target: { value: "editing" } });
  fireEvent.change(screen.getByLabelText("remark"), { target: { value: "first" } });

  rerender(
    <ResourceForm
      schema={dictTypeSchema}
      mode="update"
      resetKey="dict-type:1"
      initialValues={{ id: 1n, dictName: "A", remark: "server-refresh" }}
      onSubmit={onSubmit}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "提交" }));

  const submission = onSubmit.mock.calls[0][0];
  expect(submission.values).toMatchObject({ id: 1n, dictName: "A", remark: "server-refresh" });
  expect([...submission.touched]).toEqual([]);
  expect([...submission.dirty]).toEqual([]);
});

it("does not discard edits when a parent recreates an equal initialValues object for the same resetKey", () => {
  const onSubmit = vi.fn<(submission: ResourceFormSubmission) => void>();
  const { rerender } = render(
    <ResourceForm
      schema={dictTypeSchema}
      mode="update"
      resetKey="dict-type:1"
      initialValues={{ id: 1n, dictName: "A", remark: "first" }}
      onSubmit={onSubmit}
    />,
  );
  fireEvent.change(screen.getByLabelText("remark"), { target: { value: "editing" } });

  rerender(
    <ResourceForm
      schema={dictTypeSchema}
      mode="update"
      resetKey="dict-type:1"
      initialValues={{ id: 1n, dictName: "A", remark: "first" }}
      onSubmit={onSubmit}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "提交" }));

  const submission = onSubmit.mock.calls[0][0];
  expect(submission.values.remark).toBe("editing");
  expect([...submission.touched]).toEqual(["remark"]);
  expect([...submission.dirty]).toEqual(["remark"]);
});
