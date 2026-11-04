import { createClient, type Client, type Transport } from "@connectrpc/connect";
import { AuthService } from "@abysstract/admin-proto/abysstract/admin/v1/auth_pb";
import { DictService } from "@abysstract/admin-proto/abysstract/admin/v1/dict_pb";
import { ConfigService } from "@abysstract/admin-proto/abysstract/admin/v1/config_pb";
import { NoticeService } from "@abysstract/admin-proto/abysstract/admin/v1/notice_pb";
import { LogService } from "@abysstract/admin-proto/abysstract/admin/v1/log_pb";
import { UploadService } from "@abysstract/admin-proto/abysstract/admin/v1/upload_pb";
import { CommonExportService } from "@abysstract/admin-proto/abysstract/admin/v1/common_export_pb";
import { ProfileService } from "@abysstract/admin-proto/abysstract/admin/v1/profile_pb";
import { DashboardService } from "@abysstract/admin-proto/abysstract/admin/v1/dashboard_pb";

export interface SystemClients {
  readonly auth: Client<typeof AuthService>;
  readonly dict: Client<typeof DictService>;
  readonly config: Client<typeof ConfigService>;
  readonly notice: Client<typeof NoticeService>;
  readonly log: Client<typeof LogService>;
  readonly upload: Client<typeof UploadService>;
  readonly commonExport: Client<typeof CommonExportService>;
  readonly profile: Client<typeof ProfileService>;
  readonly dashboard: Client<typeof DashboardService>;
}

export function createSystemClients(transport: Transport): SystemClients {
  return {
    auth: createClient(AuthService, transport),
    dict: createClient(DictService, transport),
    config: createClient(ConfigService, transport),
    notice: createClient(NoticeService, transport),
    log: createClient(LogService, transport),
    upload: createClient(UploadService, transport),
    commonExport: createClient(CommonExportService, transport),
    profile: createClient(ProfileService, transport),
    dashboard: createClient(DashboardService, transport),
  };
}
