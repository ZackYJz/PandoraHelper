import accountService, {AccountAddReq} from "@/api/services/accountService.ts";
import {
  Button,
  Form,
  Input,
  Modal,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  Tooltip,
  Typography
} from "antd";
import {useEffect, useState} from "react";
import {useTranslation} from "react-i18next";
import Password from "antd/es/input/Password";
import {InfoCircleOutlined} from "@ant-design/icons";
import {useQuery} from "@tanstack/react-query";

export type AccountModalProps = {
  formValue: AccountAddReq;
  title: string;
  show: boolean;
  onOk: (values: AccountAddReq, setLoading: any) => void;
  onCancel: VoidFunction;
};

export function AccountModal({ title, show, formValue, onOk, onCancel }: AccountModalProps) {
  const [form1] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const { t } = useTranslation();

  const { data, isLoading } = useQuery({
    queryKey: ['accounts', 'one-api-channel', formValue],
    queryFn: () => accountService.getOneApiChannelList(),
    enabled: show,
  });


  useEffect(() => {
    if (show) {
      form1.setFieldsValue(formValue)
    }
  }, [show, formValue, form1]);

  const onModalOk = () => {
    form1.validateFields().then((values) => {
      setLoading(true);
      onOk(values, setLoading);
    });
  };

  const onClickSessionKey = () => {
    Modal.info({
      title: 'Session Key 获取方法',
      content: (
        <ul>
          <li>1. <Button type={'link'} href={'https://demo.fuclaude.com/'} target={'_blank'}>点击登录 Fuclaude </Button> </li>
          <li>2. <Button type={'link'} href={'https://demo.fuclaude.com/api/auth/session'} target={'_blank'}>点击获取SessionKey</Button></li>
        </ul>
      ),
    })
  }

  return (
    <Modal
      title={title}
      open={show}
      onOk={onModalOk}
      onCancel={() => {
        form1.resetFields();
        onCancel();
      }}
      okButtonProps={{
        loading,
      }}
      destroyOnClose={true}
    >
      <Form
        initialValues={formValue}
        form={form1}
        layout="vertical"
        preserve={false}
        autoComplete="off"
      >
        <Form.Item<AccountAddReq> name="id" hidden>
          <Input />
        </Form.Item>
        <Form.Item<AccountAddReq> name="accountType" hidden>
          <Input />
        </Form.Item>
        <Form.Item<AccountAddReq> label="Email" name="email" required>
          <Input placeholder={"仅作标记用, 没有实际用处"} />
        </Form.Item>
        <Form.Item<AccountAddReq> label={t('token.password')} name="password">
          <Password placeholder={"仅作标记用, 没有实际用处"} />
        </Form.Item>
        <Form.Item<AccountAddReq>
          label={
            <Space>
              共享
              <Tooltip title={"开启后，将分享在 /share 页面，任何人都可以使用它"} >
                <InfoCircleOutlined/>
              </Tooltip>
            </Space>
          }
          name="shared"
          labelAlign="left"
          valuePropName="checked"
          getValueFromEvent={(v) => {
            return v ? 1 : 0;
          }}
          required

        >
          <Switch />
        </Form.Item>
        {formValue.accountType === 'chatgpt' ? (
          <Form.Item<AccountAddReq>
            label={"OneAPI 渠道"}
            name={"oneApiChannelId"}
          >
            <Select
              placeholder={"对接OneApi/NewApi的渠道, 非必填"}
              allowClear={true}
              loading={isLoading}
              notFoundContent={isLoading ? <Spin size={"small"} /> : null}
            >
              {data?.map((item) => (
                <Select.Option key={item.id}>
                  <Space>
                    <Typography.Text>{item.name}</Typography.Text>
                    <div>
                      {
                        item.group.split(',').map((group) => {
                            const colors = ["volcano", "orange", "gold", "lime", "green", "cyan", "blue", "geekblue", "purple", "magenta", "red"];
                            // 根据group随机hash出不同颜色
                            return <Tag color={colors[group.charCodeAt(0) % colors.length]}
                            key={group}
                            >{group}</Tag>
                        })
                      }
                    </div>
                  </Space>
                </Select.Option>
              ))}
            </Select>
          </Form.Item>
        ): null}
        {formValue.accountType === 'chatgpt' ? (
          <>
            <Form.Item
              label={
                <a href="https://token.oaifree.com/auth" target="_blank" rel="noopener noreferrer">
                  Refresh Token (点击获取)
                </a>
              }
              name="refreshToken"
            >
              <Input.TextArea />
            </Form.Item>
            <Form.Item
              label={
                <a href="https://token.oaifree.com/auth" target="_blank" rel="noopener noreferrer">
                  Access Token (点击获取)
                </a>
              }
              name="accessToken"
            >
              <Input.TextArea />
            </Form.Item>
          </>
        ) : (
          <Form.Item
            label={
              <Space>
                Session Key
                <Button type={'link'} onClick={onClickSessionKey}>
                  获取方法
                </Button>
              </Space>
            }
            name="sessionKey"
          >
            <Input.TextArea />
          </Form.Item>
        )}
      </Form>
    </Modal>
  );
}
