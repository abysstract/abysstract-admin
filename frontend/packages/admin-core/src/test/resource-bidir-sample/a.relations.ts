import { createRelationOptionLoader } from "../../components/resource/relation";
import { bidirRawOperations } from "./registry";

export const aRelationLoaders = {
  b: createRelationOptionLoader<bigint>({
    remoteSearchEnabled: false,
    async loadPage(input) {
      const response = await bidirRawOperations.B.list(input);
      const nextPage =
        BigInt(response.page.pageNum) * BigInt(response.page.pageSize) < response.page.total
          ? response.page.pageNum + 1
          : undefined;
      const options = response.items.map((item) => ({ value: item.id, label: item.label }));
      return nextPage === undefined ? { options } : { options, nextPage };
    },
  }),
} as const;
