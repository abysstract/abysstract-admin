import type { Transport } from "@connectrpc/connect";
import { createDictDataRelationLoaders } from "../gen/resource/dict-data.relations.gen";
import { createResourceOperations } from "../gen/resource/resource-operations.gen";
import { configBuildUpdateRequest } from "../gen/resource/config.operations.gen";
import { dictDataBuildUpdateRequest } from "../gen/resource/dict-data.operations.gen";
import { dictTypeBuildUpdateRequest } from "../gen/resource/dict-type.operations.gen";
import { noticeBuildUpdateRequest } from "../gen/resource/notice.operations.gen";
import { deptBuildUpdateRequest } from "../gen/resource/dept.operations.gen";
import { menuBuildUpdateRequest } from "../gen/resource/menu.operations.gen";
import { roleBuildUpdateRequest } from "../gen/resource/role.operations.gen";
import { userBuildUpdateRequest } from "../gen/resource/user.operations.gen";

export function createSystemOperations(transport: Transport) {
  const created = createResourceOperations(transport);
  return {
    raw: created.raw,
    operations: created.operations,
    resourceRaw: created.raw,
    resourceOperations: created.operations,
    configRaw: created.raw.Config,
    dictDataRaw: created.raw.DictData,
    dictTypeRaw: created.raw.DictType,
    noticeRaw: created.raw.Notice,
    deptRaw: created.raw.Dept,
    menuRaw: created.raw.Menu,
    roleRaw: created.raw.Role,
    userRaw: created.raw.User,
    configOperations: created.operations.Config,
    dictDataOperations: created.operations.DictData,
    dictTypeOperations: created.operations.DictType,
    noticeOperations: created.operations.Notice,
    deptOperations: created.operations.Dept,
    menuOperations: created.operations.Menu,
    roleOperations: created.operations.Role,
    userOperations: created.operations.User,
    dictDataRelationLoaders: createDictDataRelationLoaders({
      DictType: created.raw.DictType,
    }),
    configBuildUpdateRequest,
    dictDataBuildUpdateRequest,
    dictTypeBuildUpdateRequest,
    noticeBuildUpdateRequest,
    deptBuildUpdateRequest,
    menuBuildUpdateRequest,
    roleBuildUpdateRequest,
    userBuildUpdateRequest,
  };
}

export type SystemOperations = ReturnType<typeof createSystemOperations>;
