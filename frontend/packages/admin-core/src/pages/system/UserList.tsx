import type {
  UserCreateValues,
  UserFilters,
  UserUpdateValues,
} from "../../gen/resource/user.operations.gen";
import { useSystemClients, useSystemOperations } from "../../system/hooks";
import { useEffect, useState } from "react";
import {
  Button,
  Col,
  Form,
  Input,
  Modal,
  Popconfirm,
  Result,
  Row,
  Select,
  Switch,
  Upload,
} from "antd";
import { PageContainer } from "@ant-design/pro-components";
import type { User } from "@abysstract/admin-proto/abysstract/admin/v1/user_pb";
import { Status } from "@abysstract/admin-proto/abysstract/admin/v1/common_pb";
import { deptSchema } from "../../gen/resource/dept.schema.gen";
import { roleSchema } from "../../gen/resource/role.schema.gen";
import { userSchema } from "../../gen/resource/user.schema.gen";
import {
  ResourceForm,
  ResourceFormDrawer,
  ResourceTable,
  resourceListRows,
} from "../../components/resource";
import type {
  ResourceColumnOverrides,
  ResourceCustomRowActionOverride,
  ResourceCustomToolbarOverride,
  ResourceFormItemOverrides,
  ResourceListResult,
  ResourceRowActionContext,
  ResourceToolbarContext,
} from "../../components/resource";
import { statusTag, timeCell } from "../../util";
import { useAccess } from "../../runtime/access";
import { exportExcel, importExcel } from "../../utils/batchActions";
import { formatErrorMessage, useAppFeedback, useAppMessage } from "../../runtime/message";
import { useAdminTexts } from "../../runtime/texts";

type UserCreateItems = ResourceFormItemOverrides<
  typeof userSchema,
  UserCreateValues,
  "create",
  User
>;
type UserUpdateItems = ResourceFormItemOverrides<
  typeof userSchema,
  UserUpdateValues,
  "update",
  User
>;
type UserRoleOption = { readonly label: string; readonly value: bigint };
type UserDeptOption = { readonly label: string; readonly value: bigint };

type UserRoleFieldProps = {
  readonly value: readonly bigint[];
  readonly onChange: (value: readonly bigint[]) => void;
  readonly validationError?: string;
};

type UserOptionListQuery = {
  readonly pageNum: number;
  readonly pageSize: number;
};

const USER_OPTION_PAGE_SIZE = 100;

async function loadAllUserOptions<T extends { readonly id: bigint }>(
  list: (query: UserOptionListQuery) => Promise<ResourceListResult<T>>,
): Promise<readonly T[]> {
  const items: T[] = [];
  let pageNum = 1;
  let total = 0n;
  do {
    const result = await list({ pageNum, pageSize: USER_OPTION_PAGE_SIZE });
    const pageItems = resourceListRows(result);
    items.push(...pageItems);
    total = result.total;
    if (pageItems.length === 0) {
      if (BigInt(items.length) < total) {
        throw new Error("选项列表返回不完整");
      }
      break;
    }
    pageNum += 1;
  } while (BigInt(items.length) < total);
  return items;
}

function useRoleOptions(enabled: boolean) {
  const { roleOperations } = useSystemOperations();
  const [options, setOptions] = useState<readonly UserRoleOption[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | undefined>();

  useEffect(() => {
    if (!enabled) return;
    let active = true;
    setLoading(true);
    setError(undefined);
    void loadAllUserOptions(roleOperations.list)
      .then((roles) => {
        if (!active) return;
        setOptions(roles.map((role) => ({ label: role.roleName, value: role.id })));
      })
      .catch((cause: unknown) => {
        if (active) setError(cause instanceof Error ? cause : new Error(String(cause)));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [enabled]);

  return { options, loading, error };
}

function useDeptOptions(enabled: boolean) {
  const { deptOperations } = useSystemOperations();
  const [options, setOptions] = useState<readonly UserDeptOption[]>([
    { label: "未分配", value: 0n },
  ]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | undefined>();

  useEffect(() => {
    if (!enabled) return;
    let active = true;
    setLoading(true);
    setError(undefined);
    void loadAllUserOptions(deptOperations.list)
      .then((depts) => {
        if (!active) return;
        setOptions([
          { label: "未分配", value: 0n },
          ...depts.map((dept) => ({ label: dept.deptName, value: dept.id })),
        ]);
      })
      .catch((cause: unknown) => {
        if (active) setError(cause instanceof Error ? cause : new Error(String(cause)));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [enabled]);

  return { options, loading, error };
}

function UserRoleField({ value, onChange, validationError }: UserRoleFieldProps) {
  const { options, loading, error: loadError } = useRoleOptions(true);
  const help =
    validationError ??
    (loadError === undefined ? undefined : formatErrorMessage(loadError, "无法加载角色"));
  return (
    <Form.Item
      label="角色"
      required
      validateStatus={help === undefined ? undefined : "error"}
      help={help}
    >
      <Select
        aria-label="角色"
        mode="multiple"
        loading={loading}
        value={value.map(String)}
        options={options.map((option) => ({ label: option.label, value: String(option.value) }))}
        onChange={(nextValue: unknown) => {
          const values = Array.isArray(nextValue) ? nextValue : [];
          onChange(values.map((item) => BigInt(String(item))));
        }}
      />
    </Form.Item>
  );
}

function UserDeptSelect({
  value,
  disabled,
  onChange,
}: {
  readonly value: bigint | undefined;
  readonly disabled?: boolean;
  readonly onChange: (value: bigint | undefined) => void;
}) {
  const { options, loading, error } = useDeptOptions(true);
  return (
    <Select
      aria-label="所属部门"
      value={value === undefined ? undefined : String(value)}
      disabled={disabled}
      loading={loading}
      showSearch
      optionFilterProp="label"
      status={error === undefined ? undefined : "error"}
      notFoundContent={error === undefined ? "没有部门" : formatErrorMessage(error, "无法加载部门")}
      options={options.map((option) => ({ label: option.label, value: String(option.value) }))}
      onChange={(nextValue: unknown) =>
        onChange(
          nextValue === undefined || nextValue === "" ? undefined : BigInt(String(nextValue)),
        )
      }
    />
  );
}

function UserCreateDeptField({
  value,
  disabled,
  onChange,
}: {
  readonly value: bigint | undefined;
  readonly disabled?: boolean;
  readonly onChange: (value: bigint | undefined) => void;
}) {
  return <UserDeptSelect value={value} disabled={disabled} onChange={onChange} />;
}

function UserUpdateDeptField({
  value,
  disabled,
  onChange,
}: {
  readonly value: bigint;
  readonly disabled?: boolean;
  readonly onChange: (value: bigint) => void;
}) {
  return (
    <UserDeptSelect
      value={value}
      disabled={disabled}
      onChange={(nextValue) => {
        if (nextValue !== undefined) onChange(nextValue);
      }}
    />
  );
}

function validateEmail(value: string | undefined) {
  if (value === undefined || value === "" || /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(value)) return;
  throw new Error("邮箱格式不对，例如 name@example.com");
}

function validatePhone(value: string | undefined) {
  if (value === undefined || value === "" || /^1[3-9][0-9]{9}$/.test(value)) return;
  throw new Error("手机号应为 11 位数字");
}

const userCreateItems: UserCreateItems = {
  username: { label: "用户名" },
  nickname: { label: "昵称", sameRowAsPrevious: true },
  email: { label: "邮箱", rules: [{ validator: validateEmail }] },
  phone: { label: "手机号", sameRowAsPrevious: true, rules: [{ validator: validatePhone }] },
  deptId: {
    label: "所属部门",
    valueType: "custom",
    component: UserCreateDeptField,
    span: 12,
  },
  remark: { label: "备注", valueType: "textarea", fieldProps: { rows: 2 } },
};

const userUpdateItems: UserUpdateItems = {
  nickname: { label: "昵称" },
  email: { label: "邮箱", rules: [{ validator: validateEmail }] },
  phone: { label: "手机号", sameRowAsPrevious: true, rules: [{ validator: validatePhone }] },
  deptId: {
    label: "所属部门",
    valueType: "custom",
    component: UserUpdateDeptField,
    span: 12,
  },
  remark: { label: "备注", valueType: "textarea", fieldProps: { rows: 2 } },
};

function userColumns(canEdit: boolean): ResourceColumnOverrides<typeof userSchema, User> {
  return {
    id: { title: "ID", render: ({ value }) => String(value), hidden: true },
    username: { title: "用户名" },
    nickname: { title: "昵称" },
    email: { title: "邮箱" },
    phone: { title: "手机号" },
    status: {
      title: "状态",
      render: ({ entity, reload }) =>
        canEdit ? <UserStatusSwitch entity={entity} reload={reload} /> : statusTag(entity.status),
    },
    deptId: { title: "所属部门", render: ({ value }) => String(value), hidden: true },
    remark: { title: "备注", hidden: true },
    createTime: { title: "创建时间", render: ({ value }) => timeCell(value), hidden: true },
    updateTime: { title: "更新时间", render: ({ value }) => timeCell(value), hidden: true },
  };
}

type UserStatusSwitchProps = Pick<ResourceRowActionContext<User, bigint>, "entity" | "reload">;

// 列表里的启用/停用立即生效，所以用 Switch（见 DESIGN.md「控件选择」）。
function UserStatusSwitch({ entity, reload }: UserStatusSwitchProps) {
  const { userOperations } = useSystemOperations();
  const message = useAppMessage();
  const [loading, setLoading] = useState(false);
  const enabled = entity.status === Status.ENABLED;

  const changeStatus = async (checked: boolean) => {
    setLoading(true);
    try {
      await userOperations.custom.changeUserStatus({
        id: entity.id,
        status: checked ? Status.ENABLED : Status.DISABLED,
      });
      message.success(checked ? "已启用" : "已停用");
      await reload();
    } catch (cause) {
      message.error(formatErrorMessage(cause, "无法更改状态，请稍后重试"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Switch
      aria-label={`${entity.username}状态`}
      checked={enabled}
      loading={loading}
      disabled={loading}
      onChange={(checked) => void changeStatus(checked)}
    />
  );
}

function UserCreateAction({ reload }: Pick<ResourceToolbarContext<User, bigint>, "reload">) {
  const { userOperations } = useSystemOperations();
  const message = useAppMessage();
  const texts = useAdminTexts();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [password, setPassword] = useState("");
  const [roleIds, setRoleIds] = useState<readonly bigint[]>([]);
  const [passwordError, setPasswordError] = useState<string>();
  const [roleError, setRoleError] = useState<string>();

  const close = () => {
    setOpen(false);
    setPassword("");
    setRoleIds([]);
    setPasswordError(undefined);
    setRoleError(undefined);
  };

  const submit = async ({ values }: { values: Readonly<UserCreateValues> }) => {
    const nextPasswordError = password.trim() === "" ? "请填写密码" : undefined;
    const nextRoleError = roleIds.length === 0 ? "至少选择一个角色" : undefined;
    setPasswordError(nextPasswordError);
    setRoleError(nextRoleError);
    if (nextPasswordError !== undefined || nextRoleError !== undefined) {
      return;
    }
    setLoading(true);
    try {
      await userOperations.custom.createUser({
        ...values,
        password,
        roleIds: [...roleIds],
      });
      message.success(texts.feedback.created);
      close();
      await reload();
    } catch (cause) {
      // 抽屉不关，用户填的内容保留。
      message.error(formatErrorMessage(cause, "无法创建用户，请稍后重试"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <Button type="primary" onClick={() => setOpen(true)}>
        {texts.action.create}
      </Button>
      <ResourceFormDrawer open={open} title="新增用户" onClose={close}>
        {(actionsContainer) => (
          <ResourceForm<
            typeof userSchema,
            UserCreateValues,
            "create",
            bigint,
            UserCreateValues,
            UserUpdateValues,
            keyof UserUpdateValues,
            User
          >
            schema={userSchema}
            mode="create"
            initialValues={{
              username: "",
              nickname: "",
              email: "",
              phone: "",
              deptId: 0n,
              remark: "",
            }}
            itemOverrides={userCreateItems}
            extraFields={
              <Row gutter={16}>
                <Col span={12}>
                  <Form.Item
                    label="密码"
                    required
                    validateStatus={passwordError === undefined ? undefined : "error"}
                    help={passwordError}
                  >
                    <Input.Password
                      aria-label="密码"
                      value={password}
                      onChange={(event) => {
                        setPassword(event.currentTarget.value);
                        setPasswordError(undefined);
                      }}
                    />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <UserRoleField
                    value={roleIds}
                    validationError={roleError}
                    onChange={(nextValue) => {
                      setRoleIds(nextValue);
                      setRoleError(undefined);
                    }}
                  />
                </Col>
              </Row>
            }
            loading={loading}
            onCreate={submit}
            onCancel={close}
            actionsContainer={actionsContainer}
          />
        )}
      </ResourceFormDrawer>
    </>
  );
}

type UserTarget = { readonly entity: User; readonly reload: () => Promise<void> };

function UserEditDrawer({
  target,
  onClose,
}: {
  readonly target: UserTarget | undefined;
  readonly onClose: () => void;
}) {
  const { userOperations, userBuildUpdateRequest } = useSystemOperations();
  const message = useAppMessage();
  const texts = useAdminTexts();
  const [loading, setLoading] = useState(false);
  const [user, setUser] = useState<User | undefined>();
  const [roleIds, setRoleIds] = useState<readonly bigint[]>([]);
  const [loadError, setLoadError] = useState<Error | undefined>();
  const [attempt, setAttempt] = useState(0);
  const id = target?.entity.id;

  useEffect(() => {
    setUser(undefined);
    setRoleIds([]);
    setLoadError(undefined);
    if (id === undefined) return;
    let active = true;
    setLoading(true);
    void userOperations.custom
      .getUser({ id })
      .then((response) => {
        if (!active) return;
        if (response.user === undefined) throw new Error("用户详情缺少 user");
        setUser(response.user);
        setRoleIds(response.user.roleIds ?? []);
      })
      .catch((cause: unknown) => {
        if (!active) return;
        setLoadError(cause instanceof Error ? cause : new Error(String(cause)));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [id, attempt]);

  const submit = async ({
    values,
    touched,
  }: {
    values: Readonly<UserUpdateValues>;
    touched: ReadonlySet<keyof UserUpdateValues>;
  }) => {
    if (target === undefined) return;
    setLoading(true);
    try {
      const request = userBuildUpdateRequest(target.entity.id, values, touched);
      request.roleIds = [...roleIds];
      const response = await userOperations.custom.updateUser(request);
      if (response.user === undefined) throw new Error("保存响应缺少 user");
      message.success(texts.feedback.saved);
      onClose();
      await target.reload();
    } catch (cause) {
      message.error(formatErrorMessage(cause, "无法保存，请稍后重试"));
    } finally {
      setLoading(false);
    }
  };

  const initialValues =
    user === undefined
      ? undefined
      : {
          nickname: user.nickname,
          email: user.email,
          phone: user.phone,
          deptId: user.deptId,
          remark: user.remark,
        };

  return (
    <ResourceFormDrawer
      open={target !== undefined}
      title="编辑用户"
      loading={loading && user === undefined && loadError === undefined}
      onClose={onClose}
    >
      {(actionsContainer) =>
        loadError !== undefined ? (
          <Result
            status="error"
            title="无法加载用户"
            subTitle={formatErrorMessage(loadError)}
            extra={
              <Button onClick={() => setAttempt((value) => value + 1)}>{texts.action.retry}</Button>
            }
          />
        ) : user !== undefined && initialValues !== undefined ? (
          <ResourceForm<
            typeof userSchema,
            UserUpdateValues,
            "update",
            bigint,
            UserCreateValues,
            UserUpdateValues,
            keyof UserUpdateValues,
            User
          >
            key={String(user.id)}
            schema={userSchema}
            mode="update"
            rowKey={user.id}
            resetKey={user.id}
            initialValues={initialValues}
            itemOverrides={userUpdateItems}
            extraFields={
              <Row gutter={16}>
                <Col span={12}>
                  <UserRoleField value={roleIds} onChange={setRoleIds} />
                </Col>
              </Row>
            }
            loading={loading}
            onUpdate={submit}
            onCancel={onClose}
            actionsContainer={actionsContainer}
          />
        ) : null
      }
    </ResourceFormDrawer>
  );
}

function UserResetPasswordModal({
  target,
  onClose,
}: {
  readonly target: UserTarget | undefined;
  readonly onClose: () => void;
}) {
  const { userOperations } = useSystemOperations();
  const message = useAppMessage();
  const texts = useAdminTexts();
  const [password, setPassword] = useState("");
  const [passwordError, setPasswordError] = useState<string>();
  const [loading, setLoading] = useState(false);

  const close = () => {
    setPassword("");
    setPasswordError(undefined);
    onClose();
  };

  const submit = async () => {
    if (target === undefined) return;
    if (password.trim() === "") {
      setPasswordError("请填写新密码");
      return;
    }
    setLoading(true);
    try {
      await userOperations.custom.resetPassword({ id: target.entity.id, newPassword: password });
      message.success("已重置密码");
      close();
    } catch (cause) {
      message.error(formatErrorMessage(cause, "无法重置密码，请稍后重试"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      title={target === undefined ? "重置密码" : `重置「${target.entity.username}」的密码`}
      open={target !== undefined}
      destroyOnHidden
      onCancel={close}
      footer={[
        <Button key="cancel" disabled={loading} onClick={close}>
          {texts.action.cancel}
        </Button>,
        <Button key="submit" type="primary" loading={loading} onClick={() => void submit()}>
          重置密码
        </Button>,
      ]}
    >
      <Form layout="vertical">
        <Form.Item
          label="新密码"
          required
          validateStatus={passwordError === undefined ? undefined : "error"}
          help={passwordError}
        >
          <Input.Password
            aria-label="新密码"
            value={password}
            onChange={(event) => {
              setPassword(event.currentTarget.value);
              setPasswordError(undefined);
            }}
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}

function UserBatchDeleteAction({
  selectedKeys,
  reload,
  clearSelection,
}: Pick<ResourceToolbarContext<User, bigint>, "selectedKeys" | "reload" | "clearSelection">) {
  const { userOperations } = useSystemOperations();
  const message = useAppMessage();
  const { modal } = useAppFeedback();
  const texts = useAdminTexts();
  const [loading, setLoading] = useState(false);

  const remove = async () => {
    if (selectedKeys.length === 0) return;
    setLoading(true);
    try {
      const response = await userOperations.custom.batchDeleteUsers({ ids: [...selectedKeys] });
      message.success(texts.feedback.batchDeleted(Number(response.deleted)));
      clearSelection();
      await reload();
    } catch (cause) {
      modal.error({
        title: texts.feedback.cannotDelete,
        content: formatErrorMessage(cause, "请稍后重试"),
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Popconfirm
      destroyOnHidden
      title={`删除选中的 ${selectedKeys.length} 个用户？`}
      description={texts.confirm.deleteContent}
      onConfirm={remove}
      okText={texts.action.delete}
      cancelText={texts.action.cancel}
      okButtonProps={{ danger: true, loading }}
      cancelButtonProps={{ disabled: loading }}
      disabled={selectedKeys.length === 0 || loading}
    >
      <Button danger disabled={selectedKeys.length === 0 || loading} loading={loading}>
        {texts.action.delete}
      </Button>
    </Popconfirm>
  );
}

function UserExportAction() {
  const { commonExport } = useSystemClients();
  const message = useAppMessage();
  return (
    <Button onClick={() => void exportExcel("system:user", { message, client: commonExport })}>
      导出
    </Button>
  );
}

function UserImportAction({ reload }: Pick<ResourceToolbarContext<User, bigint>, "reload">) {
  const { commonExport } = useSystemClients();
  const message = useAppMessage();
  return (
    <Upload
      accept=".xlsx,.xls"
      showUploadList={false}
      beforeUpload={(file) => {
        if (file.size > 10 * 1024 * 1024) {
          message.error("无法导入：文件超过 10MB");
          return Upload.LIST_IGNORE;
        }
        void importExcel("system:user", file, {
          onSuccess: () => void reload(),
          message,
          client: commonExport,
        });
        return false;
      }}
    >
      <Button>导入</Button>
    </Upload>
  );
}

const userListPermission = userSchema.permissions?.list ?? `${userSchema.permPrefix}:list`;
const userGetPermission = userSchema.permissions?.get ?? `${userSchema.permPrefix}:list`;
const userCreatePermission = userSchema.permissions?.create ?? `${userSchema.permPrefix}:add`;
const userUpdatePermission = userSchema.permissions?.update ?? `${userSchema.permPrefix}:edit`;
const userDeletePermission = userSchema.permissions?.delete ?? `${userSchema.permPrefix}:remove`;
const roleListPermission = roleSchema.permissions?.list ?? `${roleSchema.permPrefix}:list`;
const deptListPermission = deptSchema.permissions?.list ?? `${deptSchema.permPrefix}:list`;

type UserRowHandlers = {
  readonly edit: (target: UserTarget) => void;
  readonly resetPassword: (target: UserTarget) => void;
  readonly remove: (target: UserTarget) => void;
};

// 操作列：「编辑」露出，「重置密码」「删除」收进「⋯」（见 DESIGN.md「数据列表」）。
function userRowActions(
  canOpenEditor: boolean,
  handlers: UserRowHandlers,
): readonly ResourceCustomRowActionOverride<User, bigint>[] {
  return [
    {
      action: "custom",
      name: "edit",
      title: "编辑",
      permission: userUpdatePermission,
      merge: "append",
      // The editor performs a dedicated GetUser and loads role/dept options.
      // Do not expose a button that is guaranteed to fail in a partial role.
      hidden: !canOpenEditor,
      onClick: ({ entity, reload }) => handlers.edit({ entity, reload }),
    },
    {
      action: "custom",
      name: "resetPassword",
      title: "重置密码",
      permission: `${userSchema.permPrefix}:resetPwd`,
      merge: "append",
      onClick: ({ entity, reload }) => handlers.resetPassword({ entity, reload }),
    },
    {
      action: "custom",
      name: "delete",
      title: "删除",
      permission: userDeletePermission,
      merge: "append",
      danger: true,
      onClick: ({ entity, reload }) => handlers.remove({ entity, reload }),
    },
  ];
}

// 工具栏：新增 → 导入 → 导出；批量删除在选中行后出现在表格底部。
function userToolbar(
  canOpenCreateForm: boolean,
): readonly ResourceCustomToolbarOverride<User, bigint>[] {
  return [
    {
      action: "custom",
      name: "create",
      permission: userCreatePermission,
      merge: "append",
      // The form always fetches both relation option lists when opened.
      hidden: !canOpenCreateForm,
      render: (context) => <UserCreateAction {...context} />,
    },
    {
      action: "custom",
      name: "import",
      permission: userCreatePermission,
      merge: "append",
      render: (context) => <UserImportAction {...context} />,
    },
    {
      action: "custom",
      name: "export",
      permission: userListPermission,
      merge: "append",
      render: () => <UserExportAction />,
    },
    {
      action: "custom",
      name: "batchDelete",
      permission: userDeletePermission,
      merge: "append",
      placement: "selection",
      render: (context) => <UserBatchDeleteAction {...context} />,
    },
  ];
}

export default function UserList() {
  const { userOperations } = useSystemOperations();
  const message = useAppMessage();
  const { modal } = useAppFeedback();
  const texts = useAdminTexts();
  const { has } = useAccess();
  const canEdit = has(userUpdatePermission);
  const canLoadUserRelations = has(roleListPermission) && has(deptListPermission);
  const canOpenEditor = canEdit && has(userGetPermission) && canLoadUserRelations;
  const canOpenCreateForm = has(userCreatePermission) && canLoadUserRelations;
  const [editing, setEditing] = useState<UserTarget>();
  const [resetting, setResetting] = useState<UserTarget>();

  const remove = ({ entity, reload }: UserTarget) => {
    modal.confirm({
      title: `删除用户「${entity.username}」？`,
      content: texts.confirm.deleteContent,
      okText: texts.action.delete,
      okButtonProps: { danger: true },
      cancelText: texts.action.cancel,
      onOk: async () => {
        try {
          await userOperations.custom.deleteUser({ id: entity.id });
          message.success(texts.feedback.deleted);
          await reload();
        } catch (cause) {
          modal.error({
            title: texts.feedback.cannotDelete,
            content: formatErrorMessage(cause, "请稍后重试"),
          });
        }
      },
    });
  };

  return (
    <PageContainer>
      <ResourceTable<typeof userSchema, User, bigint, UserFilters>
        readOnly
        schema={userSchema}
        operations={userOperations}
        access={{ can: (permission) => has(permission) }}
        columns={userColumns(canEdit)}
        rowActions={userRowActions(canOpenEditor, {
          edit: setEditing,
          resetPassword: setResetting,
          remove,
        })}
        toolbar={userToolbar(canOpenCreateForm)}
      />
      <UserEditDrawer target={editing} onClose={() => setEditing(undefined)} />
      <UserResetPasswordModal target={resetting} onClose={() => setResetting(undefined)} />
    </PageContainer>
  );
}
