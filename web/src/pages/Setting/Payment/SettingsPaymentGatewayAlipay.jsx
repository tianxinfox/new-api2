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
import { API, showError, showInfo, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

export default function SettingsPaymentGatewayAlipay(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    AlipayEnabled: false,
    AlipaySandbox: false,
    AlipayUseCertificateMode: false,
    AlipayPayMode: 'page',
    AlipayOrderExpireMinutes: 30,
    AlipayPendingSweepDelayMinutes: 30,
    AlipayAppID: '',
    AlipayPrivateKey: '',
    AlipayPublicKey: '',
    AlipayAppPublicCert: '',
    AlipayAlipayPublicCert: '',
    AlipayRootCert: '',
  });
  const [originInputs, setOriginInputs] = useState({});
  const formApiRef = useRef(null);

  useEffect(() => {
    if (props.options && formApiRef.current) {
      const currentInputs = {
        AlipayEnabled: props.options.AlipayEnabled || false,
        AlipaySandbox: props.options.AlipaySandbox || false,
        AlipayUseCertificateMode: props.options.AlipayUseCertificateMode || false,
        AlipayPayMode: props.options.AlipayPayMode || 'page',
        AlipayOrderExpireMinutes: props.options.AlipayOrderExpireMinutes || 30,
        AlipayPendingSweepDelayMinutes: props.options.AlipayPendingSweepDelayMinutes || 30,
        AlipayAppID: props.options.AlipayAppID || '',
        AlipayPrivateKey: props.options.AlipayPrivateKey || '',
        AlipayPublicKey: props.options.AlipayPublicKey || '',
        AlipayAppPublicCert: props.options.AlipayAppPublicCert || '',
        AlipayAlipayPublicCert: props.options.AlipayAlipayPublicCert || '',
        AlipayRootCert: props.options.AlipayRootCert || '',
      };
      setInputs(currentInputs);
      setOriginInputs({ ...currentInputs });
      formApiRef.current.setValues(currentInputs);
    }
  }, [props.options]);

  const submitAlipaySetting = async () => {
    setLoading(true);
    try {
      const options = [];
      const nextOriginInputs = { ...originInputs };
      const skippedSensitiveKeys = [];
      if (originInputs.AlipayEnabled !== inputs.AlipayEnabled) {
        options.push({
          key: 'AlipayEnabled',
          value: inputs.AlipayEnabled ? 'true' : 'false',
        });
        nextOriginInputs.AlipayEnabled = inputs.AlipayEnabled;
      }
      if (originInputs.AlipaySandbox !== inputs.AlipaySandbox) {
        options.push({
          key: 'AlipaySandbox',
          value: inputs.AlipaySandbox ? 'true' : 'false',
        });
        nextOriginInputs.AlipaySandbox = inputs.AlipaySandbox;
      }
      if (originInputs.AlipayUseCertificateMode !== inputs.AlipayUseCertificateMode) {
        options.push({
          key: 'AlipayUseCertificateMode',
          value: inputs.AlipayUseCertificateMode ? 'true' : 'false',
        });
        nextOriginInputs.AlipayUseCertificateMode = inputs.AlipayUseCertificateMode;
      }
      if (originInputs.AlipayPayMode !== inputs.AlipayPayMode) {
        options.push({
          key: 'AlipayPayMode',
          value: inputs.AlipayPayMode || 'page',
        });
        nextOriginInputs.AlipayPayMode = inputs.AlipayPayMode || 'page';
      }
      ['AlipayOrderExpireMinutes', 'AlipayPendingSweepDelayMinutes'].forEach((key) => {
        const nextValue = Number(inputs[key]);
        const prevValue = Number(originInputs[key]);
        if (!Number.isFinite(nextValue) || nextValue <= 0) {
          return;
        }
        if (nextValue !== prevValue) {
          options.push({ key, value: String(Math.floor(nextValue)) });
          nextOriginInputs[key] = Math.floor(nextValue);
        }
      });
      [
        'AlipayAppID',
        'AlipayPrivateKey',
        'AlipayPublicKey',
        'AlipayAppPublicCert',
        'AlipayAlipayPublicCert',
        'AlipayRootCert',
      ].forEach((key) => {
        const nextValue = inputs[key] ?? '';
        const prevValue = originInputs[key] ?? '';
        // Safety guard: empty value won't overwrite existing Alipay credentials.
        if (prevValue !== '' && nextValue === '') {
          skippedSensitiveKeys.push(key);
          return;
        }
        if (nextValue !== prevValue) {
          options.push({ key, value: nextValue });
          nextOriginInputs[key] = nextValue;
        }
      });
      if (skippedSensitiveKeys.length > 0) {
        showInfo(t('检测到密钥/证书为空输入，已自动跳过覆盖以保护现有配置。'));
      }
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
        setOriginInputs(nextOriginInputs);
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
        <Form.Section text={t('支付宝支付设置')}>
          <Banner
            type='warning'
            closeIcon={null}
            description={t(
              '支付宝异步回调地址请配置为：{serverAddress}/api/user/alipay/notify 与 {serverAddress}/api/subscription/alipay/notify',
              { serverAddress: props.options.ServerAddress || t('站点地址') },
            )}
          />
          <Text>
            {inputs.AlipayUseCertificateMode
              ? t('当前为证书模式：请填写应用私钥、应用公钥证书、支付宝公钥证书、支付宝根证书。')
              : t('当前为密钥模式：请填写应用私钥与支付宝公钥（RSA2）。')}
          </Text>
          <Row gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }} style={{ marginTop: 16 }}>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Switch
                field='AlipayEnabled'
                size='default'
                checkedText={t('是')}
                uncheckedText={t('否')}
                label={t('启用支付宝支付')}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Switch
                field='AlipaySandbox'
                size='default'
                checkedText={t('是')}
                uncheckedText={t('否')}
                label={t('使用沙箱环境')}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Switch
                field='AlipayUseCertificateMode'
                size='default'
                checkedText={t('是')}
                uncheckedText={t('否')}
                label={t('使用证书模式')}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Select
                field='AlipayPayMode'
                label={t('支付下单模式')}
                optionList={[
                  { label: t('页面支付 (page)'), value: 'page' },
                  { label: t('预下单二维码 (precreate)'), value: 'precreate' },
                ]}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input field='AlipayAppID' label={t('AppID')} placeholder='20**************' />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='AlipayOrderExpireMinutes'
                label={t('订单过期时间（分钟）')}
                min={1}
                step={1}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='AlipayPendingSweepDelayMinutes'
                label={t('待支付扫描延迟（分钟）')}
                min={1}
                step={1}
              />
            </Col>
          </Row>
          <Form.TextArea
            field='AlipayPrivateKey'
            label={t('应用私钥')}
            autosize
            placeholder={t('-----BEGIN PRIVATE KEY----- ... -----END PRIVATE KEY-----')}
          />
          {inputs.AlipayUseCertificateMode ? (
            <>
              <Form.TextArea
                field='AlipayAppPublicCert'
                label={t('应用公钥证书')}
                autosize
                placeholder={t('-----BEGIN CERTIFICATE----- ... -----END CERTIFICATE-----')}
              />
              <Form.TextArea
                field='AlipayAlipayPublicCert'
                label={t('支付宝公钥证书')}
                autosize
                placeholder={t('-----BEGIN CERTIFICATE----- ... -----END CERTIFICATE-----')}
              />
              <Form.TextArea
                field='AlipayRootCert'
                label={t('支付宝根证书')}
                autosize
                placeholder={t('-----BEGIN CERTIFICATE----- ... -----END CERTIFICATE-----')}
              />
            </>
          ) : (
            <Form.TextArea
              field='AlipayPublicKey'
              label={t('支付宝公钥')}
              autosize
              placeholder={t('-----BEGIN PUBLIC KEY----- ... -----END PUBLIC KEY-----')}
            />
          )}
          <Button onClick={submitAlipaySetting}>{t('更新支付宝设置')}</Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}
