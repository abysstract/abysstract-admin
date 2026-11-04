import type {
  UserCreateValues,
  UserFilters,
  UserUpdateValues,
} from "../../gen/resource/user.operations.gen";
import { useSystemClients, useSystemOperations } from "../../system/hooks";
import { useEffect, useState } from "react";
import { Button, Form, Input, Modal, Popconfirm, Select, Switch, Upload } from "antd";
import { DeleteOutlined, DownloadOutlined, PlusOutlined, UploadOutlined } from "@ant-design/icons";
import { PageContainer } from "@ant-design/pro-components";
import type { User } from "@abysstract/admin-proto/abysstract/admin/v1/user_pb";
import { Status } from "@abysstract/admin-proto/abysstract/admin/v1/common_pb";
import { deptSchema } from "../../gen/resource/dept.schema.gen";
import { roleSchema } from "../../gen/resource/role.schema.gen";
import { userSchema } from "../../gen/resource/user.schema.gen";
import { ResourceForm, ResourceTable, resourceListRows } from "../../components/resource";
import type {
  ResourceColumnOverrides,
  ResourceCustomRowActionOverride,
  ResourceCustomToolbarOverride,
  ResourceFormItemOverrides,
  ResourceListResult,
  ResourceRowActionContext,
  ResourceToolbarContext,
} from "../../components/resource";
import { fmtTime, statusTag } from "../../util";
import { useAccess } from "../../runtime/access";
import { exportExcel, importExcel } from "../../utils/batchActions";
import { formatErrorMessage, useAppMessage } from "../../runtime/message";

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
    (loadError === undefined ? undefined : formatErrorMessage(loadError, "角色加载失败"));
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
      notFoundContent={error === undefined ? "暂无部门" : formatErrorMessage(error, "暂无部门")}
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

const userCreateItems: UserCreateItems = {
  username: { label: "用户名" },
  nickname: { label: "昵称" },
  email: {
    label: "邮箱",
    rules: [
      {
        validator: (value) => {
          if (value === undefined || value === "" || /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(value))
            return;
          throw new Error("邮箱格式不正确");
        },
      },
    ],
  },
  phone: {
    label: "手机号",
    rules: [
      {
        validator: (value) => {
          if (value === undefined || value === "" || /^1[3-9][0-9]{9}$/.test(value)) return;
          throw new Error("手机号格式不正确");
        },
      },
    ],
  },
  deptId: {
    label: "所属部门",
    valueType: "custom",
    component: UserCreateDeptField,
  },
  remark: { label: "备注", valueType: "textarea", fieldProps: { rows: 3 } },
};

const userUpdateItems: UserUpdateItems = {
  nickname: { label: "昵称" },
  email: {
    label: "邮箱",
    rules: [
      {
        validator: (value) => {
          if (value === undefined || value === "" || /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(value))
            return;
          throw new Error("邮箱格式不正确");
        },
      },
    ],
  },
  phone: {
    label: "手机号",
    rules: [
      {
        validator: (value) => {
          if (value === undefined || value === "" || /^1[3-9][0-9]{9}$/.test(value)) return;
          throw new Error("手机号格式不正确");
        },
      },
    ],
  },
  deptId: {
    label: "所属部门",
    valueType: "custom",
    component: UserUpdateDeptField,
  },
  remark: { label: "备注", valueType: "textarea", fieldProps: { rows: 3 } },
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
    createTime: { title: "创建时间", render: ({ value }) => fmtTime(value), hidden: true },
    updateTime: { title: "更新时间", render: ({ value }) => fmtTime(value), hidden: true },
  };
}

type UserStatusSwitchProps = Pick<ResourceRowActionContext<User, bigint>, "entity" | "reload">;

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
      message.success(checked ? "用户已启用" : "用户已停用");
      await reload();
    } catch (cause) {
      message.error(formatErrorMessage(cause, "状态更新失败"));
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
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [password, setPassword] = useState("");
  const [roleIds, setRoleIds] = useState<readonly bigint[]>([]);
  const [passwordError, setPasswordError] = useState<string>();
  const [roleError, setRoleError] = useState<string>();
  const [error, setError] = useState<Error | undefined>();

  const submit = async ({ values }: { values: Readonly<UserCreateValues> }) => {
    const nextPasswordError = password.trim() === "" ? "请输入密码" : undefined;
    const nextRoleError = roleIds.length === 0 ? "请至少选择一个角色" : undefined;
    setError(undefined);
    setPasswordError(nextPasswordError);
    setRoleError(nextRoleError);
    if (nextPasswordError !== undefined || nextRoleError !== undefined) {
      return;
    }
    setLoading(true);
    setError(undefined);
    try {
      await userOperations.custom.createUser({
        ...values,
        password,
        roleIds: [...roleIds],
      });
      message.success("创建成功");
      setOpen(false);
      setPassword("");
      setRoleIds([]);
      setPasswordError(undefined);
      setRoleError(undefined);
      await reload();
    } catch (cause) {
      const nextError = cause instanceof Error ? cause : new Error(String(cause));
      setError(nextError);
      message.error(formatErrorMessage(nextError, "创建失败"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <Button type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>
        新建用户
      </Button>
      <Modal
        title="新建用户"
        open={open}
        footer={null}
        destroyOnHidden
        onCancel={() => {
          setOpen(false);
          setPassword("");
          setRoleIds([]);
          setPasswordError(undefined);
          setRoleError(undefined);
          setError(undefined);
        }}
      >
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
            <>
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
                    setError(undefined);
                  }}
                />
              </Form.Item>
              <UserRoleField
                value={roleIds}
                validationError={roleError}
                onChange={(nextValue) => {
                  setRoleIds(nextValue);
                  setRoleError(undefined);
                  setError(undefined);
                }}
              />
            </>
          }
          error={error}
          loading={loading}
          onCreate={submit}
        />
      </Modal>
    </>
  );
}

type UserEditActionProps = Pick<ResourceRowActionContext<User, bigint>, "entity" | "reload">;

function UserEditAction({ entity, reload }: UserEditActionProps) {
  const { userOperations, userBuildUpdateRequest } = useSystemOperations();
  const message = useAppMessage();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [user, setUser] = useState<User | undefined>();
  const [roleIds, setRoleIds] = useState<readonly bigint[]>([]);
  const [error, setError] = useState<Error | undefined>();

  const openEditor = () => {
    setUser(undefined);
    setRoleIds([]);
    setError(undefined);
    setOpen(true);
  };

  useEffect(() => {
    if (!open) return;
    let active = true;
    setLoading(true);
    setError(undefined);
    void userOperations.custom
      .getUser({ id: entity.id })
      .then((response) => {
        if (!active) return;
        if (response.user === undefined) throw new Error("用户详情缺少 user");
        setUser(response.user);
        setRoleIds(response.user.roleIds ?? []);
      })
      .catch((cause: unknown) => {
        if (!active) return;
        const nextError = cause instanceof Error ? cause : new Error(String(cause));
        setError(nextError);
        message.error(formatErrorMessage(nextError, "加载用户失败"));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [entity.id, message, open]);

  const submit = async ({
    values,
    touched,
  }: {
    values: Readonly<UserUpdateValues>;
    touched: ReadonlySet<keyof UserUpdateValues>;
  }) => {
    setLoading(true);
    setError(undefined);
    try {
      const request = userBuildUpdateRequest(entity.id, values, touched);
      request.roleIds = [...roleIds];
      const response = await userOperations.custom.updateUser(request);
      if (response.user === undefined) throw new Error("保存响应缺少 user");
      message.success("已保存");
      setOpen(false);
      await reload();
    } catch (cause) {
      const nextError = cause instanceof Error ? cause : new Error(String(cause));
      setError(nextError);
      message.error(formatErrorMessage(nextError, "保存失败"));
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
    <>
      <Button type="link" autoInsertSpace={false} onClick={openEditor}>
        编辑
      </Button>
      <Modal
        title="编辑用户"
        open={open}
        loading={loading && user === undefined}
        footer={null}
        destroyOnHidden
        onCancel={() => {
          setOpen(false);
          setUser(undefined);
          setRoleIds([]);
          setError(undefined);
        }}
      >
        {error !== undefined && user === undefined ? (
          <div role="alert">{formatErrorMessage(error)}</div>
        ) : null}
        {user !== undefined && initialValues !== undefined ? (
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
              <UserRoleField
                value={roleIds}
                onChange={(nextValue) => {
                  setRoleIds(nextValue);
                  setError(undefined);
                }}
              />
            }
            error={error}
            loading={loading}
            onUpdate={submit}
          />
        ) : null}
      </Modal>
    </>
  );
}

function UserResetPasswordAction({ entity }: UserEditActionProps) {
  const { userOperations } = useSystemOperations();
  const message = useAppMessage();
  const [open, setOpen] = useState(false);
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);

  const submit = async () => {
    if (password.trim() === "") {
      message.error("请输入新密码");
      return;
    }
    setLoading(true);
    try {
      await userOperations.custom.resetPassword({ id: entity.id, newPassword: password });
      message.success("密码已重置");
      setOpen(false);
      setPassword("");
    } catch (cause) {
      message.error(formatErrorMessage(cause, "重置失败"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <Button
        type="link"
        autoInsertSpace={false}
        onClick={() => {
          setPassword("");
          setOpen(true);
        }}
      >
        改密
      </Button>
      <Modal
        title="重置密码"
        open={open}
        footer={null}
        destroyOnHidden
        onCancel={() => {
          setOpen(false);
          setPassword("");
        }}
      >
        <Form.Item label="新密码" required>
          <Input.Password
            aria-label="新密码"
            value={password}
            onChange={(event) => setPassword(event.currentTarget.value)}
          />
        </Form.Item>
        <Button type="primary" loading={loading} onClick={() => void submit()}>
          提交
        </Button>
      </Modal>
    </>
  );
}

function UserDeleteAction({ entity, reload }: UserEditActionProps) {
  const { userOperations } = useSystemOperations();
  const message = useAppMessage();
  const [loading, setLoading] = useState(false);

  const remove = async () => {
    setLoading(true);
    try {
      await userOperations.custom.deleteUser({ id: entity.id });
      message.success("已删除");
      await reload();
    } catch (cause) {
      message.error(formatErrorMessage(cause, "删除失败"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Popconfirm title="确认删除该用户？" onConfirm={() => void remove()}>
      <Button type="link" danger loading={loading} autoInsertSpace={false}>
        删除
      </Button>
    </Popconfirm>
  );
}

function UserBatchDeleteAction({
  selectedKeys,
  reload,
  clearSelection,
}: Pick<ResourceToolbarContext<User, bigint>, "selectedKeys" | "reload" | "clearSelection">) {
  const { userOperations } = useSystemOperations();
  const message = useAppMessage();
  const [loading, setLoading] = useState(false);

  const remove = async () => {
    if (selectedKeys.length === 0) return;
    setLoading(true);
    try {
      const response = await userOperations.custom.batchDeleteUsers({ ids: [...selectedKeys] });
      message.success(`已删除 ${response.deleted} 条`);
      clearSelection();
      await reload();
    } catch (cause) {
      message.error(formatErrorMessage(cause, "批量删除失败"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Popconfirm
      destroyOnHidden
      title={`确认删除选中的 ${selectedKeys.length} 个用户？`}
      onConfirm={remove}
      okText="确认删除"
      cancelText="取消"
      okButtonProps={{ danger: true, loading }}
      cancelButtonProps={{ disabled: loading }}
      disabled={selectedKeys.length === 0 || loading}
    >
      <Button
        danger
        icon={<DeleteOutlined />}
        disabled={selectedKeys.length === 0 || loading}
        loading={loading}
      >
        批量删除（{selectedKeys.length}）
      </Button>
    </Popconfirm>
  );
}

function UserExportAction() {
  const { commonExport } = useSystemClients();
  const message = useAppMessage();
  return (
    <Button
      icon={<DownloadOutlined />}
      onClick={() => void exportExcel("system:user", { message, client: commonExport })}
    >
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
          message.error("导入文件不能超过 10MB");
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
      <Button icon={<UploadOutlined />}>导入</Button>
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

function userRowActions(
  canOpenEditor: boolean,
): readonly ResourceCustomRowActionOverride<User, bigint>[] {
  return [
    {
      action: "custom",
      name: "edit",
      permission: userUpdatePermission,
      merge: "append",
      // The editor performs a dedicated GetUser and loads role/dept options.
      // Do not expose a button that is guaranteed to fail in a partial role.
      hidden: !canOpenEditor,
      render: (context) => <UserEditAction {...context} />,
    },
    {
      action: "custom",
      name: "resetPassword",
      permission: `${userSchema.permPrefix}:resetPwd`,
      merge: "append",
      render: (context) => <UserResetPasswordAction {...context} />,
    },
    {
      action: "custom",
      name: "delete",
      permission: userDeletePermission,
      merge: "append",
      render: (context) => <UserDeleteAction {...context} />,
    },
  ];
}

function userToolbar(
  canOpenCreateForm: boolean,
): readonly ResourceCustomToolbarOverride<User, bigint>[] {
  return [
    {
      action: "custom",
      name: "batchDelete",
      permission: userDeletePermission,
      merge: "append",
      hidden: ({ selectedKeys }) => selectedKeys.length === 0,
      render: (context) => <UserBatchDeleteAction {...context} />,
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
      name: "import",
      permission: userCreatePermission,
      merge: "append",
      render: (context) => <UserImportAction {...context} />,
    },
    {
      action: "custom",
      name: "create",
      permission: userCreatePermission,
      merge: "append",
      // The form always fetches both relation option lists when opened.
      hidden: !canOpenCreateForm,
      render: (context) => <UserCreateAction {...context} />,
    },
  ];
}

export default function UserList() {
  const { userOperations } = useSystemOperations();
  const message = useAppMessage();
  const { has } = useAccess();
  const canEdit = has(userUpdatePermission);
  const canLoadUserRelations = has(roleListPermission) && has(deptListPermission);
  const canOpenEditor = canEdit && has(userGetPermission) && canLoadUserRelations;
  const canOpenCreateForm = has(userCreatePermission) && canLoadUserRelations;

  return (
    <PageContainer>
      <ResourceTable<typeof userSchema, User, bigint, UserFilters>
        readOnly
        schema={userSchema}
        operations={userOperations}
        access={{ can: (permission) => has(permission) }}
        onError={(error) => message.error(formatErrorMessage(error))}
        columns={userColumns(canEdit)}
        rowActions={userRowActions(canOpenEditor)}
        toolbar={userToolbar(canOpenCreateForm)}
      />
    </PageContainer>
  );
}
