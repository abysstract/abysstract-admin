import type { BidirListInput, BidirListResponse } from "./a.raw";

export const bRaw = {
  async list(input: BidirListInput): Promise<BidirListResponse> {
    void input;
    return {
      items: [{ id: 2n, label: "B" }],
      page: { total: 1n, pageNum: 1, pageSize: 10 },
    };
  },
} as const;
