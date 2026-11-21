import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

import { create } from "@bufbuild/protobuf";
import type {
  DictData,
  ListDictDataRequest,
} from "@abysstract/admin-proto/abysstract/admin/v1/dict_pb";
import { DictDataSchema } from "@abysstract/admin-proto/abysstract/admin/v1/dict_pb";
import { dictDataSchema } from "../../gen/resource/dict-data.schema.gen";
import { ResourceTable } from "./ResourceTable";
import type { ResourceReadOperations } from "./operations";
import type { SortableFieldName } from "./schema";

type Filters = Pick<ListDictDataRequest, "dictType">;
type OrderField = SortableFieldName<typeof dictDataSchema>;

const row = create(DictDataSchema, {
  id: 7n,
  dictSort: 1,
  dictLabel: "启用",
  dictValue: "enabled",
  dictType: "notice",
});

function Table() {
  const operations: ResourceReadOperations<DictData, bigint, Filters, OrderField> = {
    keyOf: (entity) => entity.id,
    list: vi.fn().mockResolvedValue({ items: [row], total: 1n, pageNum: 1, pageSize: 10 }),
  };
  return (
    <ResourceTable<typeof dictDataSchema, DictData, bigint, Filters>
      readOnly
      schema={dictDataSchema}
      operations={operations}
      access={{ can: () => true }}
      rowActions={[
        {
          action: "custom",
          name: "view",
          title: "查看",
          permission: "system:dict:query",
          merge: "append",
          href: ({ entity }) => `/dict/detail/${entity.dictValue}`,
        },
      ]}
    />
  );
}

function Location() {
  return <span data-testid="location">{useLocation().pathname}</span>;
}

// The test setup replaces antd's Button with a plain element; the real Button
// renders an <a> when it has an href.
describe("ResourceTable link row actions", () => {
  it("renders a real anchor that navigates in-app on a plain click", async () => {
    render(
      <MemoryRouter initialEntries={["/dict"]}>
        <Routes>
          <Route path="/dict" element={<Table />} />
          <Route path="/dict/detail/:value" element={<Location />} />
        </Routes>
      </MemoryRouter>,
    );
    const link = (await screen.findByText("查看")).closest("[href]") as HTMLElement;
    expect(link).toHaveAttribute("href", "/dict/detail/enabled");

    // Modified clicks are left to the browser (new tab), not routed in-app.
    fireEvent.click(link, { ctrlKey: true });
    fireEvent.click(link, { metaKey: true });
    expect(screen.queryByTestId("location")).not.toBeInTheDocument();

    fireEvent.click(link);
    expect(await screen.findByTestId("location")).toHaveTextContent("/dict/detail/enabled");
  });

  it("keeps the href outside a router", async () => {
    render(<Table />);
    const link = (await screen.findByText("查看")).closest("[href]") as HTMLElement;
    expect(link).toHaveAttribute("href", "/dict/detail/enabled");
  });
});
