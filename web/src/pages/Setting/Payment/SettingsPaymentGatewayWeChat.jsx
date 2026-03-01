/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useRef, useState } from 'react';
import { Banner, Button, Col, Form, Row, Spin, Typography } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

export default function SettingsPaymentGatewayWeChat(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    WeChatPayEnabled: false,
    WeChatPayMchID: '',
    WeChatPayAppID: '',
    WeChatPayAPIv3Key: '',
    WeChatPayMchSerial: '',
    WeChatPayPrivateKey: '',
    WeChatNativeExpireMinutes: 5,
    WeChatDelayedCheckMinutes: 6,
  });
  const [originInputs, setOriginInputs] = useState({});
  const formApiRef = useRef(null);

  useEffect(() => {
    if (props.options && formApiRef.current) {
      const currentInputs = {
        WeChatPayEnabled: props.options.WeChatPayEnabled || false,
        WeChatPayMchID: props.options.WeChatPayMchID || '',
        WeChatPayAppID: props.options.WeChatPayAppID || '',
        WeChatPayAPIv3Key: props.options.WeChatPayAPIv3Key || '',
        WeChatPayMchSerial: props.options.WeChatPayMchSerial || '',
        WeChatPayPrivateKey: props.options.WeChatPayPrivateKey || '',
        WeChatNativeExpireMinutes: props.options.WeChatNativeExpireMinutes || 5,
        WeChatDelayedCheckMinutes: props.options.WeChatDelayedCheckMinutes || 6,
      };
      setInputs(currentInputs);
      setOriginInputs({ ...currentInputs });
      formApiRef.current.setValues(currentInputs);
    }
  }, [props.options]);

  const submitWeChatPaySetting = async () => {
    setLoading(true);
    try {
      const options = [];
      if (originInputs.WeChatPayEnabled !== inputs.WeChatPayEnabled) {
        options.push({
          key: 'WeChatPayEnabled',
          value: inputs.WeChatPayEnabled ? 'true' : 'false',
        });
      }
      const textKeys = [
        'WeChatPayMchID',
        'WeChatPayAppID',
        'WeChatPayAPIv3Key',
        'WeChatPayMchSerial',
        'WeChatPayPrivateKey',
      ];
      textKeys.forEach((key) => {
        const nextValue = inputs[key] ?? '';
        const prevValue = originInputs[key] ?? '';
        if (nextValue !== prevValue) {
          options.push({ key, value: nextValue });
        }
      });
      ['WeChatNativeExpireMinutes', 'WeChatDelayedCheckMinutes'].forEach((key) => {
        const nextValue = Number(inputs[key]);
        const prevValue = Number(originInputs[key]);
        if (!Number.isFinite(nextValue) || nextValue <= 0) {
          return;
        }
        if (nextValue !== prevValue) {
          options.push({ key, value: String(Math.floor(nextValue)) });
        }
      });
      if (options.length === 0) {
        showSuccess(t('无更新'));
        setLoading(false);
        return;
      }
      const requestQueue = options.map((opt) =>
        API.put('/api/option/', {
          key: opt.key,
          value: opt.value,
        }),
      );
      const results = await Promise.all(requestQueue);
      const errorResults = results.filter((res) => !res.data.success);
      if (errorResults.length > 0) {
        errorResults.forEach((res) => showError(res.data.message));
      } else {
        showSuccess(t('更新成功'));
        setOriginInputs({ ...inputs });
        props.refresh?.();
      }
    } catch (error) {
      showError(t('更新失败'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Spin spinning={loading}>
      <Form
        initValues={inputs}
        onValueChange={(values) => setInputs(values)}
        getFormApi={(api) => (formApiRef.current = api)}
      >
        <Form.Section text={t('微信支付设置')}>
          <Banner
            type='warning'
            closeIcon={null}
            description={t(
              '回调地址请在微信支付商户平台配置为：{serverAddress}/api/user/wechat/notify 与 {serverAddress}/api/subscription/wechat/notify',
              { serverAddress: props.options.ServerAddress || t('站点地址') },
            )}
          />
          <Text>{t('私钥支持直接粘贴 PEM 内容，支持多行格式。')}</Text>
          <Row gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }} style={{ marginTop: 16 }}>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Switch
                field='WeChatPayEnabled'
                size='default'
                checkedText={t('是')}
                uncheckedText={t('否')}
                label={t('启用微信支付')}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input field='WeChatPayMchID' label={t('商户号 MchID')} placeholder='190000****' />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input field='WeChatPayAppID' label={t('应用 AppID')} placeholder='wx*************' />
            </Col>
          </Row>
          <Row gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }} style={{ marginTop: 16 }}>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='WeChatPayAPIv3Key'
                label={t('APIv3 密钥')}
                type='password'
                placeholder={t('32位 APIv3 密钥')}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='WeChatPayMchSerial'
                label={t('商户证书序列号')}
                placeholder={t('证书序列号')}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='WeChatNativeExpireMinutes'
                label={t('二维码过期时间（分钟）')}
                min={1}
                step={1}
              />
            </Col>
          </Row>
          <Row gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }} style={{ marginTop: 16 }}>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='WeChatDelayedCheckMinutes'
                label={t('延迟检查时间（分钟）')}
                min={1}
                step={1}
              />
            </Col>
          </Row>
          <Form.TextArea
            field='WeChatPayPrivateKey'
            label={t('商户私钥 PEM')}
            autosize
            placeholder={t('-----BEGIN PRIVATE KEY----- ... -----END PRIVATE KEY-----')}
          />
          <Button onClick={submitWeChatPaySetting}>{t('更新微信支付设置')}</Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}
