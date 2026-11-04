import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { create } from "@bufbuild/protobuf";
import { ConfigSchema } from "@abysstract/admin-proto/abysstract/admin/v1/config_pb";
import { configSchema } from "../../gen/resource/config.schema.gen";
import {
  createConfigOperations,
  type ConfigUpdateValues,
} from "../../gen/resource/config.operations.gen";
import { ResourceForm } from "./ResourceForm";
import type { ResourceFormCreateProps } from "./form-contract";
import type { ResourceUpdateSubmission } from "./operations";

vi.unmock("antd");
vi.unmock("@ant-design/pro-components");

const field = {
  protoName: "",
  required: false,
  searchable: false,
  filterable: false,
  sortable: false,
  listable: true,
  maxLen: 0,
  creatable: true,
  updatable: true,
  serverManaged: false,
} as const;
const schema = {
  name: "Defaults",
  permPrefix: "defaults",
  table: "defaults",
  fields: [
    { ...field, name: "enabled", tsType: "boolean" },
    { ...field, name: "brand", tsType: "string" },
  ],
} as const;

it.each([false, true])("uses one initial context when brand visibility is %s", async (visible) => {
  const onCreate = vi.fn();
  const props: ResourceFormCreateProps<typeof schema, { enabled: boolean; brand: string }> = {
    schema,
    mode: "create",
    onCreate,
    itemOverrides: {
      brand: {
        visible: ({ entity }) => (visible ? entity?.enabled === false : entity?.enabled !== false),
      },
    },
  };
  render(<ResourceForm {...props} />);
  if (visible) expect(screen.getByLabelText("brand")).toHaveValue("");
  else expect(screen.queryByLabelText("brand")).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "提交" }));
  await waitFor(() => expect(onCreate).toHaveBeenCalledOnce());
  expect(onCreate.mock.calls[0][0].values).toEqual({
    enabled: false,
    brand: visible ? "" : undefined,
  });
});

it("keeps explicit initial values and hidden business values", async () => {
  const onCreate = vi.fn();
  const props: ResourceFormCreateProps<typeof schema, { enabled: boolean; brand: string }> = {
    schema,
    mode: "create",
    onCreate,
    initialValues: { enabled: true, brand: "业务初值" },
    itemOverrides: { brand: { visible: ({ entity }) => entity?.enabled === false } },
  };
  render(<ResourceForm {...props} />);
  expect(screen.queryByLabelText("brand")).toBeNull();
  expect(screen.getByRole("switch", { name: "enabled" })).toBeChecked();
  fireEvent.click(screen.getByRole("button", { name: "提交" }));
  await waitFor(() => expect(onCreate).toHaveBeenCalledOnce());
  expect(onCreate.mock.calls[0][0].values).toEqual({ enabled: true, brand: "业务初值" });
});

it("sends a touched field restored to its initial value through generated operations", async () => {
  const item = create(ConfigSchema, {
    id: 1n,
    configName: "原名称",
    configValue: "原值",
    remark: "保留",
  });
  const updateConfig = vi.fn(async () => ({ item }));
  const operations = createConfigOperations({ updateConfig } as never);
  const onUpdate = vi.fn(
    async (submission: ResourceUpdateSubmission<bigint, ConfigUpdateValues>) => {
      await operations.update(submission);
    },
  );
  render(
    <ResourceForm
      schema={configSchema}
      mode="update"
      rowKey={1n}
      initialValues={{
        configName: item.configName,
        configValue: item.configValue,
        remark: item.remark,
      }}
      onUpdate={onUpdate}
    />,
  );
  fireEvent.change(screen.getByLabelText("configName"), { target: { value: "改名称" } });
  fireEvent.change(screen.getByLabelText("configName"), { target: { value: item.configName } });
  fireEvent.click(screen.getByRole("button", { name: "提交" }));
  await waitFor(() => expect(updateConfig).toHaveBeenCalledOnce());
  expect(onUpdate.mock.calls[0][0].touched).toEqual(new Set(["configName"]));
  expect(onUpdate.mock.calls[0][0].dirty).toEqual(new Set());
  expect(updateConfig).toHaveBeenCalledWith({ id: 1n, configName: "原名称" });
});
