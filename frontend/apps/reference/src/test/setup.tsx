import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";

afterEach(() => {
  cleanup();
});
import type { ChangeEvent, ComponentType, ReactNode } from "react";

Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }),
});

const originalGetComputedStyle = window.getComputedStyle;
window.getComputedStyle = (elt, pseudoElt) => {
  const style = originalGetComputedStyle(elt, pseudoElt);
  return style;
};

// 全局 mock antd / @ant-design/pro-components / icons：把它们替换为保留可断言属性的
// 原生 HTML 元素，避免完整 antd + ProTable 在 jsdom 下的渲染开销（单测 30+s → <1s）。
//
// 冒烟测试的目标是「页面能挂载 + 标志性元素存在」，不需要 antd 的真实样式与交互逻辑。
// 保留：placeholder / children / 文本内容 / button 的可点击性。
//
// 如需测试 antd 真实交互（如表单校验、modal 弹出），应在专门测试中 vi.unmock。

// passThrough：渲染 children 的轻量组件。
function passThrough({ children, ...rest }: { children?: ReactNode } & Record<string, unknown>) {
  return <div {...(rest as Record<string, string>)}>{children}</div>;
}

// passthroughInput：保留 placeholder 的 input。
function passthroughInput(props: Record<string, unknown>) {
  const { children, ...rest } = props as { children?: ReactNode } & Record<string, unknown>;
  return <input {...(rest as Record<string, string>)}>{children as ReactNode[]}</input>;
}

type MockSwitchProps = {
  checked?: boolean;
  disabled?: boolean;
  id?: string;
  name?: string;
  "aria-label"?: string;
  onChange?: (checked: boolean, event: ChangeEvent<HTMLInputElement>) => void;
};

type MockUploadInfo = {
  file: File;
  fileList: File[];
};

type MockUploadProps = {
  accept?: string;
  beforeUpload?: (file: File, fileList: File[]) => boolean | string | Promise<boolean | string>;
  children?: ReactNode;
  customRequest?: (options: { file: File; onError: () => void; onSuccess: () => void }) => void;
  disabled?: boolean;
  multiple?: boolean;
  onChange?: (info: MockUploadInfo) => void;
};

// 与 antd Upload.LIST_IGNORE 保持同一引用，供页面的上传校验分支使用。
const uploadListIgnore = "__ANTD_UPLOAD_LIST_IGNORE__";

type MockTreeSelectNode = {
  title?: ReactNode;
  value?: string | number;
  children?: readonly MockTreeSelectNode[];
};

function MockTreeSelect(props: Record<string, unknown>) {
  const flatten = (nodes: readonly MockTreeSelectNode[], prefix = ""): MockTreeSelectNode[] =>
    nodes.flatMap((node) => [
      { ...node, title: `${prefix}${String(node.title ?? "")}` },
      ...(node.children === undefined ? [] : flatten(node.children, `${prefix}└ `)),
    ]);
  const options = flatten((props.treeData as MockTreeSelectNode[] | undefined) ?? []);
  const value = props.value === undefined || props.value === null ? "" : String(props.value);
  return (
    <select
      data-testid="tree-select"
      aria-label={String(props["aria-label"] ?? "")}
      value={value}
      disabled={Boolean(props.disabled)}
      onChange={(event) => {
        const option = options.find((item) => String(item.value) === event.currentTarget.value);
        (props.onChange as ((value: string | number | undefined) => void) | undefined)?.(
          option?.value ?? event.currentTarget.value,
        );
      }}
    >
      {options.map((option) => (
        <option key={String(option.value)} value={String(option.value)}>
          {option.title}
        </option>
      ))}
    </select>
  );
}

// passthroughButton：把 children 作为按钮文本。
function passthroughButton(props: Record<string, unknown>) {
  const { children, htmlType, loading, disabled, ...rest } = props as {
    children?: ReactNode;
  } & Record<string, unknown>;
  delete rest.type;
  delete rest.danger;
  delete rest.autoInsertSpace;
  const nativeType =
    htmlType === "submit" || htmlType === "reset" || htmlType === "button" ? htmlType : "button";
  return (
    <button
      type={nativeType}
      disabled={Boolean(disabled) || Boolean(loading)}
      {...(rest as Record<string, string>)}
    >
      {children}
    </button>
  );
}

vi.mock("antd", () => {
  const Form = Object.assign(
    (props: Record<string, unknown>) => <form>{props.children as ReactNode}</form>,
    {
      Item: (props: Record<string, unknown>) => (
        <div>
          {props.children as ReactNode}
          {props.help !== undefined ? <div role="alert">{props.help as ReactNode}</div> : null}
        </div>
      ),
      Group: (props: Record<string, unknown>) => <div>{props.children as ReactNode}</div>,
      useForm: () => [
        {
          resetFields() {},
          validateFields: async () => ({}),
          getFieldValue() {},
          setFieldsValue() {},
        },
      ],
    },
  );
  const Input = Object.assign(passthroughInput, {
    Password: passthroughInput,
    Search: passthroughInput,
    TextArea: passthroughInput,
  });
  const message = {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  };
  const AntdApp = Object.assign(({ children }: { children?: ReactNode }) => <>{children}</>, {
    useApp: () => ({ message }),
  });
  return {
    Button: passthroughButton,
    Space: passThrough,
    Alert: passThrough,
    Empty: ({ description }: { description?: ReactNode }) => <div>{description}</div>,
    Spin: () => <span role="status">加载中</span>,
    Modal: (props: Record<string, unknown>) =>
      props.open ? (
        <div role="dialog" aria-label={String(props.title ?? "")}>
          <button type="button" onClick={props.onCancel as (() => void) | undefined}>
            关闭
          </button>
          {props.children as ReactNode}
          {props.footer as ReactNode}
        </div>
      ) : null,
    Drawer: (props: Record<string, unknown>) =>
      props.open ? (
        <div role="dialog" aria-label={String(props.title ?? "")}>
          {props.extra as ReactNode}
          {props.children as ReactNode}
        </div>
      ) : null,
    Tree: (props: Record<string, unknown>) => (
      <div data-testid="tree">{props.children as ReactNode}</div>
    ),
    Card: passThrough,
    ConfigProvider: passThrough,
    Result: ({
      title,
      subTitle,
      extra,
    }: {
      title?: ReactNode;
      subTitle?: ReactNode;
      extra?: ReactNode;
    }) => (
      <div>
        <div>{title}</div>
        <div>{subTitle}</div>
        {extra}
      </div>
    ),
    Dropdown: ({ children }: { children?: ReactNode }) => <div>{children}</div>,
    Tag: passThrough,
    Form,
    Input,
    Popconfirm: (props: Record<string, unknown>) => <>{props.children as ReactNode}</>,
    Switch: ({
      checked,
      disabled,
      id,
      name,
      "aria-label": ariaLabel,
      onChange,
    }: MockSwitchProps) => (
      <input
        id={id}
        name={name}
        aria-label={ariaLabel}
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(event) => onChange?.(!(checked ?? false), event)}
      />
    ),
    InputNumber: (props: Record<string, unknown>) => (
      <input
        {...(props as Record<string, string>)}
        type="number"
        value={props.value === undefined || props.value === null ? "" : String(props.value)}
        onChange={(event) =>
          (props.onChange as ((value: number | undefined) => void) | undefined)?.(
            event.currentTarget.value === "" ? undefined : Number(event.currentTarget.value),
          )
        }
      />
    ),
    TreeSelect: MockTreeSelect,
    Select: (props: Record<string, unknown>) => (
      <select
        {...(props as Record<string, string>)}
        value={props.value === undefined || props.value === null ? "" : String(props.value)}
        onChange={(event) => {
          const selected = (
            props.options as Array<{ value: string | number | bigint }> | undefined
          )?.find((option) => String(option.value) === event.currentTarget.value);
          (props.onChange as ((value: string | number | bigint) => void) | undefined)?.(
            selected?.value ?? event.currentTarget.value,
          );
        }}
      >
        {(
          props.options as Array<{ value: string | number | bigint; label: string }> | undefined
        )?.map((option) => (
          <option key={String(option.value)} value={String(option.value)}>
            {option.label}
          </option>
        ))}
      </select>
    ),
    Typography: Object.assign(passThrough, {
      Title: (props: Record<string, unknown>) => <h3>{props.children as ReactNode}</h3>,
      Text: (props: Record<string, unknown>) => <span>{props.children as ReactNode}</span>,
    }),
    Upload: Object.assign(
      ({
        accept,
        beforeUpload,
        children,
        customRequest,
        disabled,
        multiple,
        onChange,
      }: MockUploadProps) => {
        const selectFiles = async (event: ChangeEvent<HTMLInputElement>) => {
          const files = Array.from(event.currentTarget.files ?? []);
          for (const file of files) {
            const beforeResult = await beforeUpload?.(file, files);
            if (beforeResult === uploadListIgnore) {
              continue;
            }
            const info = { file, fileList: files };
            onChange?.(info);
            if (beforeResult !== false) {
              customRequest?.({
                file,
                onError: () => {},
                onSuccess: () => {},
              });
            }
          }
        };

        return (
          <>
            <input
              type="file"
              accept={accept}
              disabled={disabled}
              multiple={multiple}
              onChange={(event) => void selectFiles(event)}
            />
            {children}
          </>
        );
      },
      { LIST_IGNORE: uploadListIgnore },
    ),
    App: AntdApp,
    message,
  };
});

vi.mock("@ant-design/icons", () => {
  // 所有 icon 渲染为空 span，避免 antd icon 在 jsdom 下的 SVG 渲染开销。
  // 必须显式列出所有被使用的 icon（vitest 不支持 Proxy 在 vi.mock 返回值上）。
  const Icon = () => <span />;
  return {
    default: Icon,
    DeleteOutlined: Icon,
    DownloadOutlined: Icon,
    KeyOutlined: Icon,
    LockOutlined: Icon,
    LoginOutlined: Icon,
    LogoutOutlined: Icon,
    PlusOutlined: Icon,
    TeamOutlined: Icon,
    UploadOutlined: Icon,
    UserOutlined: Icon,
    SettingOutlined: Icon,
    MenuOutlined: Icon,
    ApartmentOutlined: Icon,
    BookOutlined: Icon,
    ToolOutlined: Icon,
    NotificationOutlined: Icon,
    ProfileOutlined: Icon,
  };
});

type MockProTableColumn = {
  key?: string;
  dataIndex?: string;
  title?: ReactNode;
  sorter?: boolean | string;
  render?: (value: unknown, record: Record<string, unknown>, index: number) => ReactNode;
};
type MockProTableKey = string | number | bigint;
type MockProTableChange = (
  pagination: { current?: number; pageSize?: number },
  filters: Record<string, unknown>,
  sorter:
    | { field?: string | number; order?: "ascend" | "descend" }
    | ReadonlyArray<{ field?: string | number; order?: "ascend" | "descend" }>,
) => void;
type MockProTableProps = {
  columns?: readonly MockProTableColumn[];
  dataSource?: readonly (Record<string, unknown> & {
    children?: readonly Record<string, unknown>[];
  })[];
  rowKey?: string | ((record: Record<string, unknown>, index?: number) => MockProTableKey);
  rowSelection?: {
    selectedRowKeys?: readonly MockProTableKey[];
    onChange?: (keys: readonly MockProTableKey[]) => void;
  };
  pagination?:
    | {
        current?: number;
        pageSize?: number;
        total?: number;
        onChange?: (page: number, pageSize: number) => void;
      }
    | false;
  loading?: boolean;
  locale?: { emptyText?: ReactNode };
  search?: false | Record<string, unknown>;
  onChange?: MockProTableChange;
  toolBarRender?: () => ReactNode[];
};

function mockProTableKey(
  rowKey: MockProTableProps["rowKey"],
  record: Record<string, unknown>,
  index: number,
): MockProTableKey {
  if (typeof rowKey === "function") return rowKey(record, index);
  if (typeof rowKey === "string") return record[rowKey] as MockProTableKey;
  return index;
}

vi.mock("@ant-design/pro-components", () => {
  // This is a presentation-only ProTable double. It renders the same
  // columns/dataSource/selection/pagination props that production ProTable
  // consumes; all query and action decisions remain in ResourceTable.
  const ProTable = (rawProps: Record<string, unknown>) => {
    const props = rawProps as MockProTableProps;
    const toolBar = props.toolBarRender?.() ?? [];
    const columns = props.columns ?? [];
    const rows = props.dataSource ?? [];
    const selection = props.rowSelection;
    const selectedKeys = new Set((selection?.selectedRowKeys ?? []).map((key) => String(key)));
    const pagination = props.pagination === false ? undefined : props.pagination;
    const onChange = props.onChange;
    return (
      <div>
        <div data-testid="pro-table-toolbar">{toolBar}</div>
        {props.loading ? <div role="status">加载中</div> : null}
        {rows.length === 0 && !props.loading ? <div>{props.locale?.emptyText}</div> : null}
        <table>
          <thead>
            <tr>
              {columns.map((column) => (
                <th key={column.key ?? column.dataIndex} scope="col">
                  {column.sorter ? (
                    <button
                      type="button"
                      onClick={() =>
                        onChange?.(
                          { current: pagination?.current, pageSize: pagination?.pageSize },
                          {},
                          { field: column.dataIndex, order: "descend" },
                        )
                      }
                    >
                      {column.title}
                    </button>
                  ) : (
                    column.title
                  )}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {(() => {
              const renderRows = (
                sourceRows: readonly (Record<string, unknown> & {
                  children?: readonly Record<string, unknown>[];
                })[],
                depth = 0,
              ): ReactNode[] =>
                sourceRows.flatMap((record, index) => {
                  const key = mockProTableKey(props.rowKey, record, index);
                  const keyText = String(key);
                  const checked = selectedKeys.has(keyText);
                  const row = (
                    <tr key={keyText} data-row-key={keyText}>
                      {selection ? (
                        <td>
                          <input
                            type="checkbox"
                            aria-label={`选择 ${keyText}`}
                            checked={checked}
                            onChange={() => {
                              const next = new Set(selectedKeys);
                              if (checked) next.delete(keyText);
                              else next.add(keyText);
                              selection.onChange?.(
                                rows
                                  .map((item, itemIndex) =>
                                    mockProTableKey(props.rowKey, item, itemIndex),
                                  )
                                  .filter((itemKey) => next.has(String(itemKey))),
                              );
                            }}
                          />
                        </td>
                      ) : null}
                      {columns.map((column) => {
                        const value =
                          column.dataIndex === undefined ? undefined : record[column.dataIndex];
                        return (
                          <td key={column.key ?? column.dataIndex}>
                            {column.render
                              ? column.render(value, record, index)
                              : value === undefined || value === null
                                ? ""
                                : String(value)}
                          </td>
                        );
                      })}
                    </tr>
                  );
                  const children = record.children ?? [];
                  return [row, ...(children.length > 0 ? renderRows(children, depth + 1) : [])];
                });
              return renderRows(rows);
            })()}
          </tbody>
        </table>
        {pagination ? (
          <button
            type="button"
            onClick={() => {
              const page = (pagination.current ?? 1) + 1;
              const pageSize = pagination.pageSize ?? 10;
              if (pagination.onChange !== undefined) {
                pagination.onChange(page, pageSize);
              } else {
                onChange?.({ current: page, pageSize }, {}, {});
              }
            }}
          >
            下一页
          </button>
        ) : null}
      </div>
    );
  };
  // ModalForm：把 trigger 渲染出来（children 是表单内容，不需要真实 modal）。
  const ModalForm = (props: Record<string, unknown>) => {
    const trigger = props.trigger as ReactNode;
    return (
      <>
        {trigger}
        <div hidden>{props.children as ReactNode}</div>
      </>
    );
  };
  const ProForm = Object.assign(
    (props: Record<string, unknown>) => (
      <form
        onSubmit={(event) => {
          event.preventDefault();
          void (props.onFinish as (() => void | Promise<void>) | undefined)?.();
        }}
      >
        {props.children as ReactNode}
      </form>
    ),
    { Group: passThrough },
  );
  const ProFormText = (props: Record<string, unknown>) => (
    <input {...(props as Record<string, string>)} placeholder={props.placeholder as string} />
  );
  const ProFormSelect = (props: Record<string, unknown>) => (
    <select {...(props as Record<string, string>)} aria-label={props["aria-label"] as string}>
      {(props.options as Array<{ value: string; label: string }> | undefined)?.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  );
  // ProFormText.Password：让 Login 测试能断言「密码」placeholder。
  (ProFormText as unknown as { Password: ComponentType<Record<string, unknown>> }).Password =
    passthroughInput;
  const PageContainer = passThrough;
  const ProLayout = (props: Record<string, unknown>) => {
    const avatar = props.avatarProps as { title?: ReactNode } | undefined;
    return (
      <div>
        <div>{props.title as ReactNode}</div>
        <div>{avatar?.title}</div>
        {props.children as ReactNode}
      </div>
    );
  };
  return {
    ModalForm,
    PageContainer,
    ProForm,
    ProFormText,
    ProFormSelect,
    ProTable,
    ProLayout,
  };
});
