import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

// This suite deliberately exercises the real ProTable/antd renderer. The
// behavior gate uses the lightweight pass-through renderer for speed, while
// this file protects the production prop path from children-only regressions.
vi.unmock("@ant-design/pro-components");
vi.unmock("antd");
vi.unmock("@ant-design/icons");

import { ResourceTable } from "./ResourceTable";
import type { ResourceOperations } from "./operations";
import type { ResourceSchema } from "./schema";

interface IntegrationEntity {
  readonly id: bigint;
  readonly label: string;
  readonly sort: number;
}

type IntegrationQueryFilters = Record<never, never>;
type IntegrationCreateValues = Partial<IntegrationEntity>;
type IntegrationUpdateValues = Partial<IntegrationEntity>;

type IntegrationOrderField = "sort";

const schema = {
  name: "Integration",
  permPrefix: "system:integration",
  table: "sys_integration",
  fields: [
    {
      name: "id",
      protoName: "id",
      tsType: "bigint",
      required: false,
      searchable: false,
      filterable: false,
      sortable: false,
      listable: true,
      maxLen: 0,
      creatable: false,
      updatable: false,
      serverManaged: true,
    },
    {
      name: "label",
      protoName: "label",
      tsType: "string",
      required: true,
      searchable: true,
      filterable: false,
      sortable: false,
      listable: true,
      maxLen: 100,
      creatable: true,
      updatable: true,
      serverManaged: false,
    },
    {
      name: "sort",
      protoName: "sort",
      tsType: "number",
      required: false,
      searchable: false,
      filterable: false,
      sortable: true,
      listable: true,
      maxLen: 0,
      creatable: true,
      updatable: true,
      serverManaged: false,
    },
  ],
} as const satisfies ResourceSchema;

const entity: IntegrationEntity = {
  id: 9007199254740993n,
  label: "real protable",
  sort: 1,
};

function makeOperations(
  list: ResourceOperations<
    IntegrationEntity,
    bigint,
    IntegrationQueryFilters,
    IntegrationCreateValues,
    IntegrationUpdateValues,
    IntegrationEntity,
    void,
    IntegrationOrderField
  >["list"],
): ResourceOperations<
  IntegrationEntity,
  bigint,
  IntegrationQueryFilters,
  IntegrationCreateValues,
  IntegrationUpdateValues,
  IntegrationEntity,
  void,
  IntegrationOrderField
> {
  return {
    keyOf: (value) => value.id,
    list,
    get: vi.fn(async () => entity),
    create: vi.fn(async () => entity),
    update: vi.fn(async () => entity),
    delete: vi.fn(async () => undefined),
    batchDelete: vi.fn(async () => undefined),
  };
}

describe("ResourceTable real ProTable integration", () => {
  it("renders dataSource/columns and routes search, pagination, and sorting to operations.list", async () => {
    const list = vi
      .fn<
        ResourceOperations<
          IntegrationEntity,
          bigint,
          IntegrationQueryFilters,
          IntegrationCreateValues,
          IntegrationUpdateValues,
          IntegrationEntity,
          void,
          IntegrationOrderField
        >["list"]
      >()
      .mockResolvedValue({ items: [entity], total: 100n, pageNum: 1, pageSize: 10 });

    render(
      <ResourceTable
        schema={schema}
        operations={makeOperations(list)}
        access={{ can: () => true }}
        reload={async () => undefined}
        query={{ defaults: { pageNum: 1, pageSize: 10 } }}
      />,
    );

    expect(await screen.findByText("real protable")).toBeInTheDocument();
    const initialListCalls = list.mock.calls.length;
    expect(initialListCalls).toBe(1);
    expect(screen.getByRole("columnheader", { name: "label" })).toBeInTheDocument();
    expect(screen.getByText(entity.id.toString())).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: "1" })).toBeInTheDocument();
    expect(screen.queryByText("[object Object]")).not.toBeInTheDocument();

    fireEvent.change(screen.getByRole("textbox", { name: /keyword|关键词/i }), {
      target: { value: "remote" },
    });
    fireEvent.click(screen.getByRole("button", { name: /查询|search/i }));
    await waitFor(() => {
      expect(list).toHaveBeenCalledTimes(initialListCalls + 1);
      expect(list).toHaveBeenLastCalledWith(
        expect.objectContaining({ keyword: "remote", pageNum: 1 }),
      );
    });

    fireEvent.click(screen.getByTitle("下一页"));
    await waitFor(() => {
      expect(list).toHaveBeenCalledTimes(initialListCalls + 2);
      expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ pageNum: 2 }));
    });

    fireEvent.click(screen.getByTitle("下一页"));
    await waitFor(() => {
      expect(list).toHaveBeenCalledTimes(initialListCalls + 3);
      expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ pageNum: 3 }));
    });

    fireEvent.click(screen.getByRole("columnheader", { name: "sort" }));
    await waitFor(() => {
      expect(list).toHaveBeenLastCalledWith(
        expect.objectContaining({ orders: [{ field: "sort", desc: false }] }),
      );
    });
  });

  it("preserves default orders through ProTable pagination", async () => {
    const list = vi
      .fn<
        ResourceOperations<
          IntegrationEntity,
          bigint,
          IntegrationQueryFilters,
          IntegrationCreateValues,
          IntegrationUpdateValues,
          IntegrationEntity,
          void,
          IntegrationOrderField
        >["list"]
      >()
      .mockResolvedValue({ items: [entity], total: 20n, pageNum: 1, pageSize: 10 });

    render(
      <ResourceTable
        schema={schema}
        operations={makeOperations(list)}
        access={{ can: () => true }}
        reload={async () => undefined}
        query={{
          defaults: {
            pageNum: 1,
            pageSize: 10,
            orders: [{ field: "sort", desc: true }],
          },
        }}
      />,
    );

    await screen.findByText("real protable");
    expect(list).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByTitle("下一页"));
    await waitFor(() => {
      expect(list).toHaveBeenCalledTimes(2);
      expect(list).toHaveBeenLastCalledWith({
        pageNum: 2,
        pageSize: 10,
        orders: [{ field: "sort", desc: true }],
      });
    });
  });

  it("restores query default orders after the user clears a column sort", async () => {
    const list = vi
      .fn<
        ResourceOperations<
          IntegrationEntity,
          bigint,
          IntegrationQueryFilters,
          IntegrationCreateValues,
          IntegrationUpdateValues,
          IntegrationEntity,
          void,
          IntegrationOrderField
        >["list"]
      >()
      .mockResolvedValue({ items: [entity], total: 1n, pageNum: 1, pageSize: 10 });

    render(
      <ResourceTable
        schema={schema}
        operations={makeOperations(list)}
        access={{ can: () => true }}
        reload={async () => undefined}
        query={{
          defaults: {
            pageNum: 1,
            pageSize: 10,
            orders: [{ field: "sort", desc: false }],
          },
        }}
      />,
    );

    await screen.findByText("real protable");
    const sortHeader = screen.getByRole("columnheader", { name: "sort" });
    fireEvent.click(sortHeader);
    await waitFor(() =>
      expect(list).toHaveBeenLastCalledWith(
        expect.objectContaining({ orders: [{ field: "sort", desc: false }] }),
      ),
    );
    fireEvent.click(sortHeader);
    await waitFor(() =>
      expect(list).toHaveBeenLastCalledWith(
        expect.objectContaining({ orders: [{ field: "sort", desc: true }] }),
      ),
    );
    const sorter = sortHeader.querySelector(".ant-table-column-sorter");
    if (sorter === null) throw new Error("sorter element missing");
    fireEvent.click(sorter);
    await waitFor(() =>
      expect(list).toHaveBeenLastCalledWith(
        expect.objectContaining({ orders: [{ field: "sort", desc: false }] }),
      ),
    );
  });

  it("hands typed create/edit operations and reload to form integrations", async () => {
    const list = vi
      .fn<
        ResourceOperations<
          IntegrationEntity,
          bigint,
          IntegrationQueryFilters,
          IntegrationCreateValues,
          IntegrationUpdateValues,
          IntegrationEntity,
          void,
          IntegrationOrderField
        >["list"]
      >()
      .mockResolvedValue({ items: [entity], total: 1n, pageNum: 1, pageSize: 10 });
    const operations = makeOperations(list);
    const create = vi.spyOn(operations, "create");
    const update = vi.spyOn(operations, "update");
    const reload = vi.fn(async () => undefined);

    render(
      <ResourceTable
        schema={schema}
        operations={operations}
        access={{ can: () => true }}
        reload={reload}
        form={{
          create: ({ operations: createOperations, reload: formReload }) => (
            <button
              type="button"
              onClick={() =>
                void (async () => {
                  await createOperations.create({ label: "created" });
                  await formReload();
                })()
              }
            >
              create form
            </button>
          ),
          edit: ({ operations: updateOperations, key, reload: formReload }) => (
            <button
              type="button"
              onClick={() =>
                void (async () => {
                  await updateOperations.update({
                    key,
                    values: { label: "edited" },
                    touched: new Set(["label"]),
                    dirty: new Set(["label"]),
                  });
                  await formReload();
                })()
              }
            >
              edit form
            </button>
          ),
        }}
      />,
    );

    await screen.findByText("real protable");
    fireEvent.click(screen.getByRole("button", { name: "create form" }));
    await waitFor(() => expect(create).toHaveBeenCalledWith({ label: "created" }));

    fireEvent.click(screen.getByRole("button", { name: "edit form" }));
    await waitFor(() =>
      expect(update).toHaveBeenCalledWith(
        expect.objectContaining({ key: entity.id, values: { label: "edited" } }),
      ),
    );
    expect(typeof update.mock.calls[0]?.[0].key).toBe("bigint");
  });
});

it("keeps real checkbox keys aligned and sends lossless bigint batch identifiers", async () => {
  const operations = makeOperations(
    vi.fn(async () => ({ items: [entity], total: 1n, pageNum: 1, pageSize: 10 })),
  );
  render(<ResourceTable schema={schema} operations={operations} access={{ can: () => true }} />);
  const cell = await screen.findByText(entity.label);
  const tableRow = cell.closest("tr");
  if (tableRow === null) throw new Error("entity row is missing");
  const checkbox = within(tableRow).getByRole("checkbox");
  fireEvent.click(checkbox);
  await waitFor(() => expect(checkbox).toBeChecked());
  const batch = screen.getByRole("button", { name: "批量删除" });
  expect(batch).toBeEnabled();
  fireEvent.click(batch);
  const dialog = await screen.findByRole("dialog", { name: "确认批量删除" });
  fireEvent.click(within(dialog).getByRole("button", { name: "确认批量删除" }));
  await waitFor(() => expect(operations.batchDelete).toHaveBeenCalledWith([entity.id]));
  await waitFor(() => expect(checkbox).not.toBeChecked());
});
