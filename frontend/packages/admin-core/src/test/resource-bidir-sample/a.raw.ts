export type BidirListInput = { pageNum: number; pageSize: number; keyword?: string };
export type BidirListResponse = {
  items: ReadonlyArray<{ id: bigint; label: string }>;
  page: { total: bigint; pageNum: number; pageSize: number };
};

export const aRaw = {
  async list(input: BidirListInput): Promise<BidirListResponse> {
    void input;
    return {
      items: [{ id: 1n, label: "A" }],
      page: { total: 1n, pageNum: 1, pageSize: 10 },
    };
  },
} as const;
