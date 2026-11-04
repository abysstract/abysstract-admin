import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { dictDataSchema } from "../../gen/resource/dict-data.schema.gen";
import { ResourceTable } from "./ResourceTable";
import type { ResourceSchema } from "./schema";

describe("ResourceTable", () => {
  it("renders only listable generated schema columns and keeps bigint display lossless", () => {
    const schema: ResourceSchema = {
      ...dictDataSchema,
      fields: dictDataSchema.fields.map((field) =>
        field.name === "cssClass" ? { ...field, listable: false } : field,
      ),
    };
    render(
      <ResourceTable
        schema={schema}
        rows={[
          {
            id: 9007199254740993n,
            dictLabel: "标签",
            dictValue: "value",
            dictSort: 0,
            cssClass: "hidden",
          },
        ]}
        rowKey={(row) => String(row.id)}
      />,
    );

    expect(screen.getByRole("columnheader", { name: "id" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "dictLabel" })).toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: "cssClass" })).not.toBeInTheDocument();
    expect(screen.getByText("9007199254740993")).toBeInTheDocument();
  });
});

it("uses the caller-supplied rowKey instead of assuming an id field", () => {
  const rowKey = vi.fn((row: Readonly<Record<string, unknown>>) => String(row.code));
  render(
    <ResourceTable
      schema={dictDataSchema}
      rows={[{ code: "business-key", dictLabel: "标签" }]}
      rowKey={rowKey}
    />,
  );

  expect(rowKey).toHaveBeenCalledWith(expect.objectContaining({ code: "business-key" }), 0);
});
