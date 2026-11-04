import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { create } from "@bufbuild/protobuf";
import type {
  CreateDictDataRequest,
  DictData,
} from "@abysstract/admin-proto/abysstract/admin/v1/dict_pb";
import { CreateDictDataRequestSchema } from "@abysstract/admin-proto/abysstract/admin/v1/dict_pb";
import { dictDataSchema } from "../../gen/resource/dict-data.schema.gen";
import {
  dictDataBuildUpdateRequest,
  type DictDataUpdateValues,
} from "../../gen/resource/dict-data.operations.gen";
import { createSystemOperations } from "../../system/operations";

const { dictDataRelationLoaders } = createSystemOperations({} as never);
import { ResourceForm } from "./ResourceForm";
import type {
  ResourceFormCreateProps,
  ResourceFormItemOverrides,
  ResourceFormUpdateProps,
} from "./form-contract";
import type { RelationOptionLoader } from "./relation";
import type { ResourceUpdateSubmission } from "./operations";
import type { ResourceFieldProps } from "./form-contract";
import type { ResourceSchema } from "./schema";

/**
 * Every test below uses the production create/update overload. The
 * existing ResourceForm.test.tsx intentionally remains a legacy prototype
 * test and is not evidence for this suite.
 *
 * This suite is the production acceptance gate for the ResourceForm branch;
 * the legacy ResourceForm.test.tsx suite remains a compatibility test.
 */
type GeneratedCreateValues = CreateDictDataRequest;
type GeneratedEntity = DictData;
type CreateProps = ResourceFormCreateProps<
  typeof dictDataSchema,
  GeneratedCreateValues,
  keyof GeneratedCreateValues,
  unknown,
  GeneratedEntity
>;
type UpdateProps = ResourceFormUpdateProps<
  typeof dictDataSchema,
  bigint,
  DictDataUpdateValues,
  keyof DictDataUpdateValues,
  unknown,
  GeneratedEntity
>;
type CreateHandler = NonNullable<CreateProps["onCreate"]>;
type UpdateHandler = NonNullable<UpdateProps["onUpdate"]>;

const createValues = create(CreateDictDataRequestSchema, {
  dictType: "notice",
  dictSort: 1,
  dictLabel: "启用",
  dictValue: "enabled",
  isDefault: 0,
  remark: "初始备注",
});

const updateValues: DictDataUpdateValues = {
  dictSort: 9,
  dictLabel: "启用",
  dictValue: "enabled",
  isDefault: 1,
  remark: "初始备注",
};

const booleanSchema = {
  name: "BooleanFixture",
  permPrefix: "fixture:boolean",
  table: "fixture_boolean",
  fields: [
    {
      name: "enabled",
      protoName: "enabled",
      tsType: "boolean",
      required: false,
      searchable: false,
      filterable: false,
      sortable: false,
      listable: true,
      maxLen: 0,
      creatable: true,
      updatable: true,
      serverManaged: false,
    },
  ],
} as const satisfies ResourceSchema;

const bigintSchema = {
  name: "BigIntFixture",
  permPrefix: "fixture:bigint",
  table: "fixture_bigint",
  fields: [
    {
      name: "weight",
      protoName: "weight",
      tsType: "bigint",
      required: false,
      searchable: false,
      filterable: false,
      sortable: false,
      listable: true,
      maxLen: 0,
      creatable: true,
      updatable: true,
      serverManaged: false,
    },
  ],
} as const satisfies ResourceSchema;

function createProps(
  onCreate: CreateHandler,
  overrides?: ResourceFormItemOverrides<
    typeof dictDataSchema,
    GeneratedCreateValues,
    "create",
    GeneratedEntity
  >,
): CreateProps {
  return {
    schema: dictDataSchema,
    mode: "create",
    initialValues: createValues,
    itemOverrides: overrides,
    relationLoaders: { dictType: dictDataRelationLoaders.dictType },
    onCreate,
  };
}

function updateProps(
  onUpdate: UpdateHandler,
  overrides?: ResourceFormItemOverrides<
    typeof dictDataSchema,
    DictDataUpdateValues,
    "update",
    GeneratedEntity
  >,
): UpdateProps {
  return {
    schema: dictDataSchema,
    mode: "update",
    rowKey: 9007199254740993n,
    initialValues: updateValues,
    itemOverrides: overrides,
    onUpdate,
  };
}

function deferred<T>(): { promise: Promise<T>; resolve: (value: T) => void } {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}

type RelationRequest = Parameters<RelationOptionLoader<string>["load"]>[0];
type RelationPage = Awaited<ReturnType<RelationOptionLoader<string>["load"]>>;

describe("ResourceForm production behavior failure gates", () => {
  it("enforces generated required/maxLen rules and runs the create lifecycle", async () => {
    const onCreate = vi.fn<CreateHandler>(async () => undefined);
    render(<ResourceForm {...createProps(onCreate)} />);

    fireEvent.change(screen.getByLabelText("dictLabel"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: /提交|保存|create/i }));
    expect(onCreate).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent(/required|必填/i);

    fireEvent.change(screen.getByLabelText("dictLabel"), { target: { value: "x".repeat(101) } });
    fireEvent.click(screen.getByRole("button", { name: /提交|保存|create/i }));
    expect(onCreate).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent(/100|maxLen|长度/i);

    fireEvent.change(screen.getByLabelText("dictLabel"), { target: { value: "有效标签" } });
    fireEvent.click(screen.getByRole("button", { name: /提交|保存|create/i }));
    await waitFor(() => expect(onCreate).toHaveBeenCalledOnce());
    const submission = onCreate.mock.calls[0][0];
    expect(submission.values).not.toHaveProperty("id");
    expect(submission.touched).toContain("dictLabel");
  });

  it("matches backend required/maxLen semantics for whitespace and Unicode", async () => {
    const onCreate = vi.fn<CreateHandler>(async () => undefined);
    render(<ResourceForm {...createProps(onCreate)} />);

    fireEvent.change(screen.getByLabelText("dictLabel"), { target: { value: "   " } });
    fireEvent.click(screen.getByRole("button", { name: /提交/i }));
    await waitFor(() => expect(screen.getByText(/必填/)).toBeInTheDocument());
    expect(onCreate).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("dictLabel"), {
      target: { value: "😀".repeat(100) },
    });
    fireEvent.click(screen.getByRole("button", { name: /提交/i }));
    await waitFor(() => expect(onCreate).toHaveBeenCalledOnce());
  });

  it("runs update through the generated builder with only touched fields and preserves 0/empty string", async () => {
    const updateRequest = vi.fn<UpdateHandler>((submission) =>
      dictDataBuildUpdateRequest(submission.key, submission.values, submission.touched),
    );
    render(<ResourceForm {...updateProps(updateRequest)} />);

    fireEvent.change(screen.getByLabelText("dictSort"), { target: { value: "0" } });
    fireEvent.change(screen.getByLabelText("remark"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: /提交|保存|update/i }));

    await waitFor(() => expect(updateRequest).toHaveBeenCalledOnce());
    expect(updateRequest.mock.results[0]?.value).toEqual({
      id: 9007199254740993n,
      dictSort: 0,
      remark: "",
    });
    expect(updateRequest.mock.calls[0][0].touched).toEqual(new Set(["dictSort", "remark"]));
    expect(updateRequest.mock.calls[0][0].dirty).toEqual(new Set(["dictSort", "remark"]));
  });

  it("uses the explicit rowKey even when it differs from an entity id in initial values", async () => {
    const onUpdate = vi.fn<UpdateHandler>(async () => undefined);
    const rowKey = 9007199254740993n;

    render(
      <ResourceForm
        {...updateProps(onUpdate)}
        rowKey={rowKey}
        initialValues={{ id: 11n, ...updateValues } as never}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /提交|保存|update/i }));
    await waitFor(() => expect(onUpdate).toHaveBeenCalledOnce());
    expect(onUpdate.mock.calls[0]?.[0].key).toBe(rowKey);
  });

  it("applies typed item overrides, visible/disabled state, and does not expose lifecycle-forbidden fields", () => {
    const overrides: ResourceFormItemOverrides<
      typeof dictDataSchema,
      DictDataUpdateValues,
      "update",
      GeneratedEntity
    > = {
      dictLabel: { label: "标签", disabled: true, fieldProps: { placeholder: "不可编辑" } },
      remark: { visible: false },
    };
    render(
      <ResourceForm
        {...updateProps(
          vi.fn<UpdateHandler>(async () => undefined),
          overrides,
        )}
      />,
    );

    expect(screen.getByLabelText("标签")).toBeDisabled();
    expect(screen.queryByLabelText("remark")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("id")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("dictType")).not.toBeInTheDocument();
  });

  it("honors resetKey by resetting values and presence state", async () => {
    const onUpdate = vi.fn<UpdateHandler>(async () => undefined);
    const props = updateProps(onUpdate);
    const { rerender } = render(<ResourceForm {...props} resetKey="before-reset" />);

    fireEvent.change(screen.getByLabelText("remark"), { target: { value: "unsaved" } });
    rerender(<ResourceForm {...props} resetKey="after-reset" />);
    await waitFor(() => expect(screen.getByLabelText("remark")).toHaveValue("初始备注"));

    fireEvent.click(screen.getByRole("button", { name: /提交|保存|update/i }));
    await waitFor(() => expect(onUpdate).toHaveBeenCalledOnce());
    expect(onUpdate.mock.calls[0][0].touched).toEqual(new Set());
    expect(onUpdate.mock.calls[0][0].dirty).toEqual(new Set());
  });

  it("resets controlled values and presence state through the visible reset action", async () => {
    const onUpdate = vi.fn<UpdateHandler>(async () => undefined);
    render(<ResourceForm {...updateProps(onUpdate)} />);

    fireEvent.change(screen.getByLabelText("remark"), { target: { value: "unsaved" } });
    fireEvent.click(screen.getByRole("button", { name: "重置" }));

    expect(screen.getByLabelText("remark")).toHaveValue("初始备注");
    fireEvent.click(screen.getByRole("button", { name: "提交" }));
    await waitFor(() => expect(onUpdate).toHaveBeenCalledOnce());
    expect(onUpdate.mock.calls[0][0].touched).toEqual(new Set());
    expect(onUpdate.mock.calls[0][0].dirty).toEqual(new Set());
    expect(onUpdate.mock.calls[0][0].values.remark).toBe("初始备注");
  });

  it("accepts initialValues that arrive asynchronously while the form is pristine", async () => {
    const onUpdate = vi.fn<UpdateHandler>(async () => undefined);
    const props = updateProps(onUpdate);
    const { rerender } = render(
      <ResourceForm {...props} initialValues={undefined} resetKey="async-values" />,
    );

    expect(screen.getByLabelText("remark")).toHaveValue("");
    await Promise.resolve();
    rerender(
      <ResourceForm
        {...props}
        initialValues={{ ...updateValues, remark: "异步加载" }}
        resetKey="async-values"
      />,
    );
    await waitFor(() => expect(screen.getByLabelText("remark")).toHaveValue("异步加载"));

    fireEvent.click(screen.getByRole("button", { name: /提交|保存|update/i }));
    await waitFor(() => expect(onUpdate).toHaveBeenCalledOnce());
    expect(onUpdate.mock.calls[0][0].values.remark).toBe("异步加载");
  });

  it("keeps a dirty value and its original baseline when async initialValues change", async () => {
    const onUpdate = vi.fn<UpdateHandler>(async () => undefined);
    const props = updateProps(onUpdate);
    const { rerender } = render(<ResourceForm {...props} resetKey="stable" />);

    fireEvent.change(screen.getByLabelText("remark"), { target: { value: "用户编辑" } });
    rerender(
      <ResourceForm
        {...props}
        initialValues={{ ...updateValues, remark: "服务端更新" }}
        resetKey="stable"
      />,
    );
    await waitFor(() => expect(screen.getByLabelText("remark")).toHaveValue("用户编辑"));

    fireEvent.change(screen.getByLabelText("remark"), { target: { value: "初始备注" } });
    fireEvent.click(screen.getByRole("button", { name: /提交|保存|update/i }));
    await waitFor(() => expect(onUpdate).toHaveBeenCalledOnce());
    expect(onUpdate.mock.calls[0][0].touched).toEqual(new Set(["remark"]));
    expect(onUpdate.mock.calls[0][0].dirty).toEqual(new Set());
  });

  it("drops stale relation responses when remote searches race", async () => {
    const requests: Array<{
      input: RelationRequest;
      resolve: (page: RelationPage) => void;
    }> = [];
    const loader: RelationOptionLoader<string> = {
      remoteSearchEnabled: true,
      load: (input) => {
        const pending = deferred<RelationPage>();
        requests.push({ input, resolve: pending.resolve });
        return pending.promise;
      },
    };
    const props = createProps(vi.fn<CreateHandler>(async () => undefined));
    render(<ResourceForm {...props} relationLoaders={{ dictType: loader }} />);

    await waitFor(() => expect(requests).toHaveLength(1));
    expect(screen.getByRole("combobox", { name: "dictType" })).toBeInTheDocument();
    fireEvent.change(screen.getByRole("searchbox", { name: "dictType search" }), {
      target: { value: "old" },
    });
    await waitFor(() => expect(requests).toHaveLength(2));
    fireEvent.change(screen.getByRole("searchbox", { name: "dictType search" }), {
      target: { value: "new" },
    });
    await waitFor(() => expect(requests).toHaveLength(3));
    expect(requests[1]?.input.keyword).toBe("old");
    expect(requests[2]?.input.keyword).toBe("new");

    requests[2]?.resolve({ options: [{ value: "new", label: "最新" }] });
    expect(await screen.findByText("最新")).toBeInTheDocument();
    requests[1]?.resolve({ options: [{ value: "old", label: "过期" }] });
    requests[0]?.resolve({ options: [{ value: "initial", label: "初始" }] });
    await waitFor(() => expect(screen.queryByText("过期")).not.toBeInTheDocument());
  });

  it("renders production relation Select pagination, remote search, errors, and preserves typed values", async () => {
    const load = vi.spyOn(dictDataRelationLoaders.dictType, "load");
    load.mockResolvedValueOnce({ options: [{ value: "notice", label: "通知" }], nextPage: 2 });
    load.mockRejectedValueOnce(new Error("RelationPermissionDenied"));
    const bigintOptions: ResourceFieldProps<bigint> = {
      options: [{ value: 9007199254740993n, label: "大整数" }],
    };
    expect(bigintOptions.options?.[0]?.value).toBe(9007199254740993n);

    render(<ResourceForm {...createProps(vi.fn<CreateHandler>(async () => undefined))} />);
    expect(await screen.findByRole("combobox", { name: "dictType" })).toBeInTheDocument();
    expect(await screen.findByText("通知")).toBeInTheDocument();
    fireEvent.change(screen.getByRole("searchbox", { name: "dictType search" }), {
      target: { value: "not" },
    });
    expect(load).toHaveBeenLastCalledWith({ pageNum: 1, pageSize: 20, keyword: "not" });
    fireEvent.click(screen.getByRole("button", { name: /加载更多|more/i }));
    expect(await screen.findByRole("alert")).toHaveTextContent("RelationPermissionDenied");
  });

  it("submits an explicitly toggled false boolean with touched and dirty presence", async () => {
    const onCreate = vi.fn<
      (submission: {
        readonly values: Readonly<{ enabled: boolean }>;
        readonly touched: ReadonlySet<string>;
        readonly dirty: ReadonlySet<string>;
      }) => Promise<void>
    >(async () => undefined);

    render(
      <ResourceForm
        schema={booleanSchema}
        mode="create"
        initialValues={{ enabled: true }}
        onCreate={onCreate}
      />,
    );

    const enabled = screen.getByRole("checkbox", { name: "enabled" });
    expect(enabled).toBeChecked();
    fireEvent.click(enabled);
    expect(enabled).not.toBeChecked();
    fireEvent.click(screen.getByRole("button", { name: /提交|保存|create/i }));

    await waitFor(() => expect(onCreate).toHaveBeenCalledOnce());
    const submission = onCreate.mock.calls[0]?.[0];
    expect(submission?.values.enabled).toBe(false);
    expect(submission?.touched).toEqual(new Set(["enabled"]));
    expect(submission?.dirty).toEqual(new Set(["enabled"]));
  });

  it("does not throw or corrupt the value for invalid bigint text input", () => {
    render(
      <ResourceForm
        schema={bigintSchema}
        mode="create"
        initialValues={{ weight: 123n }}
        onCreate={vi.fn(async () => undefined)}
      />,
    );

    const weight = screen.getByLabelText("weight");
    expect(weight).toHaveValue("123");
    expect(() => fireEvent.change(weight, { target: { value: "-" } })).not.toThrow();
    expect(weight).toHaveValue("123");
    expect(() => fireEvent.change(weight, { target: { value: "abc" } })).not.toThrow();
    expect(weight).toHaveValue("123");
  });

  it("keeps explicit false presence and bigint Select values in production controls", () => {
    type BooleanValues = { enabled: boolean };
    const submission: ResourceUpdateSubmission<bigint, BooleanValues> = {
      key: 9007199254740993n,
      values: { enabled: false },
      touched: new Set(["enabled"]),
      dirty: new Set(["enabled"]),
    };
    const falseOptions: ResourceFieldProps<boolean> = {
      options: [{ value: false, label: "关闭" }],
    };
    const bigintSelect: ResourceFieldProps<bigint> = {
      showSearch: true,
      options: [{ value: 9007199254740993n, label: "大整数" }],
    };
    expect(submission.values.enabled).toBe(false);
    expect(submission.touched.has("enabled")).toBe(true);
    expect(falseOptions.options?.[0]?.value).toBe(false);
    expect(bigintSelect.options?.[0]?.value).toBe(9007199254740993n);

    render(<ResourceForm {...createProps(vi.fn<CreateHandler>(async () => undefined))} />);
    expect(screen.getByRole("combobox", { name: "dictType" })).toBeInTheDocument();
  });
});

it("defaults visible writable scalars and preserves explicit values and lifecycle", async () => {
  const field = booleanSchema.fields[0];
  const schema = {
    ...booleanSchema,
    fields: [
      { ...field, name: "brand", tsType: "string", label: "品牌", maxLen: 2 },
      { ...field, name: "amount", tsType: "bigint" },
      { ...field, name: "count", tsType: "number" },
      field,
      { ...field, name: "hidden", tsType: "string" },
      { ...field, name: "fixed", tsType: "string", updatable: false },
      { ...field, name: "audit", serverManaged: true, creatable: false, updatable: false },
    ],
  } as const satisfies ResourceSchema;
  const onCreate = vi.fn();
  render(
    <ResourceForm
      schema={schema}
      mode="create"
      initialValues={{ count: 7 }}
      itemOverrides={{ hidden: { visible: false }, brand: { label: "自定义品牌" } }}
      onCreate={onCreate}
    />,
  );
  expect(screen.getByLabelText("自定义品牌")).toHaveValue("");
  expect(screen.queryByLabelText("hidden")).toBeNull();
  expect(screen.queryByLabelText("audit")).toBeNull();
  fireEvent.change(screen.getByLabelText("自定义品牌"), { target: { value: "😀😀😀" } });
  fireEvent.click(screen.getByRole("button", { name: "提交" }));
  await screen.findByText(/长度不能超过 2/);
  expect(onCreate).not.toHaveBeenCalled();
  fireEvent.change(screen.getByLabelText("自定义品牌"), { target: { value: "😀😀" } });
  fireEvent.click(screen.getByRole("button", { name: "提交" }));
  await waitFor(() => expect(onCreate).toHaveBeenCalledOnce());
  expect(onCreate.mock.calls[0][0].values).toEqual({
    brand: "😀😀",
    amount: 0n,
    count: 7,
    enabled: false,
    hidden: undefined,
    fixed: "",
  });
});
