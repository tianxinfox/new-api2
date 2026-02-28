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

import React, { useEffect, useState, useContext, useRef } from 'react';
import {
  API,
  showError,
  showInfo,
  showSuccess,
  renderQuota,
  renderQuotaWithAmount,
  copy,
  getQuotaPerUnit,
} from '../../helpers';
import { Modal, Toast } from '@douyinfe/semi-ui';
import { QRCodeSVG } from 'qrcode.react';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';

import RechargeCard from './RechargeCard';
import InvitationCard from './InvitationCard';
import TransferModal from './modals/TransferModal';
import PaymentConfirmModal from './modals/PaymentConfirmModal';
import TopupHistoryModal from './modals/TopupHistoryModal';
import WeChatIcon from '../common/logo/WeChatIcon';

const WECHAT_PAY_POLLING_TIMEOUT_MS = 10 * 60 * 1000;

const TopUp = () => {
  const { t } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);

  const [redemptionCode, setRedemptionCode] = useState('');
  const [amount, setAmount] = useState(0.0);
  const [minTopUp, setMinTopUp] = useState(statusState?.status?.min_topup || 1);
  const [topUpCount, setTopUpCount] = useState(
    statusState?.status?.min_topup || 1,
  );
  const [topUpLink, setTopUpLink] = useState(
    statusState?.status?.top_up_link || '',
  );
  const [enableOnlineTopUp, setEnableOnlineTopUp] = useState(
    statusState?.status?.enable_online_topup || false,
  );
  const [priceRatio, setPriceRatio] = useState(statusState?.status?.price || 1);

  const [enableStripeTopUp, setEnableStripeTopUp] = useState(
    statusState?.status?.enable_stripe_topup || false,
  );
  const [enableWeChatTopUp, setEnableWeChatTopUp] = useState(
    statusState?.status?.enable_wechat_topup || false,
  );
  const [enableAlipayTopUp, setEnableAlipayTopUp] = useState(
    statusState?.status?.enable_alipay_topup || false,
  );
  const [statusLoading, setStatusLoading] = useState(true);

  // Creem 支付产品与弹窗状态
  const [creemProducts, setCreemProducts] = useState([]);
  const [enableCreemTopUp, setEnableCreemTopUp] = useState(false);
  const [creemOpen, setCreemOpen] = useState(false);
  const [selectedCreemProduct, setSelectedCreemProduct] = useState(null);

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [open, setOpen] = useState(false);
  const [payWay, setPayWay] = useState('');
  const [amountLoading, setAmountLoading] = useState(false);
  const [paymentLoading, setPaymentLoading] = useState(false);
  const [confirmLoading, setConfirmLoading] = useState(false);
  const [payMethods, setPayMethods] = useState([]);
  const [wechatPayOpen, setWechatPayOpen] = useState(false);
  const [wechatPayCodeUrl, setWechatPayCodeUrl] = useState('');
  const [wechatPayTradeNo, setWechatPayTradeNo] = useState('');
  const wechatPayPollingRef = useRef(null);

  const affFetchedRef = useRef(false);

  const [affLink, setAffLink] = useState('');
  const [openTransfer, setOpenTransfer] = useState(false);
  const [transferAmount, setTransferAmount] = useState(0);

  const [openHistory, setOpenHistory] = useState(false);

  const [subscriptionPlans, setSubscriptionPlans] = useState([]);
  const [subscriptionLoading, setSubscriptionLoading] = useState(true);
  const [billingPreference, setBillingPreference] =
    useState('subscription_first');
  const [activeSubscriptions, setActiveSubscriptions] = useState([]);
  const [allSubscriptions, setAllSubscriptions] = useState([]);

  // Preset top-up amount options.
  const [presetAmounts, setPresetAmounts] = useState([]);
  const [selectedPreset, setSelectedPreset] = useState(null);

  const [topupInfo, setTopupInfo] = useState({
    amount_options: [],
    discount: {},
  });

  const topUp = async () => {
    if (redemptionCode === '') {
      showInfo(t('Please enter redemption code'));
      return;
    }
    setIsSubmitting(true);
    try {
      const res = await API.post('/api/user/topup', {
        key: redemptionCode,
      });
      const { success, message, data } = res.data;
      if (success) {
        showSuccess(t('Redeemed successfully'));
        Modal.success({
          title: t('Redeemed successfully'),
          content: t('Successfully redeemed quota: ') + renderQuota(data),
          centered: true,
        });
        if (userState.user) {
          const updatedUser = {
            ...userState.user,
            quota: userState.user.quota + data,
          };
          userDispatch({ type: 'login', payload: updatedUser });
        }
        setRedemptionCode('');
      } else {
        showError(message);
      }
    } catch (err) {
      showError(t('Request failed'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const openTopUpLink = () => {
    if (!topUpLink) {
      showError(t('Top-up link is not configured'));
      return;
    }
    window.open(topUpLink, '_blank');
  };

  const preTopUp = async (payment) => {
    if (payment === 'stripe') {
      if (!enableStripeTopUp) {
        showError(t('Stripe top-up is not enabled by admin'));
        return;
      }
    } else if (payment === 'wechat') {
      if (!enableWeChatTopUp) {
        showError(t('管理员未启用微信支付'));
        return;
      }
    } else if (payment === 'alipay') {
      if (!enableAlipayTopUp) {
        showError(t('管理员未启用支付宝支付'));
        return;
      }
    } else if (!enableOnlineTopUp) {
      showError(t('Online top-up is not enabled by admin'));
      return;
    }

    setPayWay(payment);
    setPaymentLoading(true);
    try {
      if (payment === 'stripe') {
        await getStripeAmount();
      } else {
        await getAmount();
      }

      if (topUpCount < minTopUp) {
        showError(t('Top-up amount cannot be less than ') + minTopUp);
        return;
      }
      setOpen(true);
    } catch (error) {
      showError(t('Failed to get amount'));
    } finally {
      setPaymentLoading(false);
    }
  };

  const onlineTopUp = async () => {
    if (payWay === 'stripe') {
      if (amount === 0) {
        await getStripeAmount();
      }
    } else {
      if (amount === 0) {
        await getAmount();
      }
    }

    if (topUpCount < minTopUp) {
      showError(t('Top-up amount cannot be less than ') + minTopUp);
      return;
    }
    let alipayWindow = null;
    if (payWay === 'alipay') {
      alipayWindow = window.open('about:blank', '_blank');
    }
    setConfirmLoading(true);
    try {
      let res;
      if (payWay === 'stripe') {
        res = await API.post('/api/user/stripe/pay', {
          amount: parseInt(topUpCount),
          payment_method: 'stripe',
        });
      } else if (payWay === 'alipay') {
        res = await API.post('/api/user/alipay/pay', {
          amount: parseInt(topUpCount),
          payment_method: 'alipay',
        });
      } else if (payWay === 'wechat') {
        res = await API.post('/api/user/wechat/pay', {
          amount: parseInt(topUpCount),
          payment_method: 'wechat',
        });
      } else {
        res = await API.post('/api/user/pay', {
          amount: parseInt(topUpCount),
          payment_method: payWay,
        });
      }

      if (res !== undefined) {
        const { message, data } = res.data;
        if (message === 'success') {
          if (payWay === 'stripe') {
            const payLink = data?.pay_link;
            if (!payLink) {
              showError(t('支付链接不存在'));
              return;
            }
            window.open(payLink, '_blank');
          } else if (payWay === 'alipay') {
            if (!data?.pay_link) {
              if (alipayWindow) {
                alipayWindow.close();
              }
              showError(t('支付失败'));
              return;
            }
            if (alipayWindow) {
              alipayWindow.location.href = data.pay_link;
            } else {
              window.location.href = data.pay_link;
            }
          } else if (payWay === 'wechat') {
            setWechatPayCodeUrl(data.code_url || '');
            setWechatPayTradeNo(data.trade_no || '');
            setWechatPayOpen(true);
          } else {
            let params = data;
            let url = res.data.url;
            let form = document.createElement('form');
            form.action = url;
            form.method = 'POST';
            let isSafari =
              navigator.userAgent.indexOf('Safari') > -1 &&
              navigator.userAgent.indexOf('Chrome') < 1;
            if (!isSafari) {
              form.target = '_blank';
            }
            for (let key in params) {
              let input = document.createElement('input');
              input.type = 'hidden';
              input.name = key;
              input.value = params[key];
              form.appendChild(input);
            }
            document.body.appendChild(form);
            form.submit();
            document.body.removeChild(form);
          }
        } else {
          if (payWay === 'alipay' && alipayWindow) {
            alipayWindow.close();
          }
          const errorMsg =
            typeof data === 'string' ? data : message || t('Payment failed');
          showError(errorMsg);
        }
      } else {
        if (payWay === 'alipay' && alipayWindow) {
          alipayWindow.close();
        }
        showError(res);
      }
    } catch (err) {
      if (payWay === 'alipay' && alipayWindow) {
        alipayWindow.close();
      }
      console.log(err);
      showError(t('Payment request failed'));
    } finally {
      setOpen(false);
      setConfirmLoading(false);
    }
  };

  const creemPreTopUp = async (product) => {
    if (!enableCreemTopUp) {
      showError(t('Creem top-up is not enabled by admin'));
      return;
    }
    setSelectedCreemProduct(product);
    setCreemOpen(true);
  };

  const onlineCreemTopUp = async () => {
    if (!selectedCreemProduct) {
      showError(t('Please select a product'));
      return;
    }
    // Validate product has required fields
    if (!selectedCreemProduct.productId) {
      showError(t('Product configuration is invalid, please contact admin'));
      return;
    }
    setConfirmLoading(true);
    try {
      const res = await API.post('/api/user/creem/pay', {
        product_id: selectedCreemProduct.productId,
        payment_method: 'creem',
      });
      if (res !== undefined) {
        const { message, data } = res.data;
        if (message === 'success') {
          processCreemCallback(data);
        } else {
          const errorMsg =
            typeof data === 'string' ? data : message || t('Payment failed');
          showError(errorMsg);
        }
      } else {
        showError(res);
      }
    } catch (err) {
      console.log(err);
      showError(t('Payment request failed'));
    } finally {
      setCreemOpen(false);
      setConfirmLoading(false);
    }
  };

  const processCreemCallback = (data) => {
    window.open(data.checkout_url, '_blank');
  };

  const getUserQuota = async () => {
    let res = await API.get(`/api/user/self`);
    const { success, message, data } = res.data;
    if (success) {
      userDispatch({ type: 'login', payload: data });
    } else {
      showError(message);
    }
  };

  const getSubscriptionPlans = async () => {
    setSubscriptionLoading(true);
    try {
      const res = await API.get('/api/subscription/plans');
      if (res.data?.success) {
        setSubscriptionPlans(res.data.data || []);
      }
    } catch (e) {
      setSubscriptionPlans([]);
    } finally {
      setSubscriptionLoading(false);
    }
  };

  const getSubscriptionSelf = async () => {
    try {
      const res = await API.get('/api/subscription/self');
      if (res.data?.success) {
        setBillingPreference(
          res.data.data?.billing_preference || 'subscription_first',
        );
        // Active subscriptions
        const activeSubs = res.data.data?.subscriptions || [];
        setActiveSubscriptions(activeSubs);
        // All subscriptions (including expired)
        const allSubs = res.data.data?.all_subscriptions || [];
        setAllSubscriptions(allSubs);
      }
    } catch (e) {
      // ignore
    }
  };

  const updateBillingPreference = async (pref) => {
    const previousPref = billingPreference;
    setBillingPreference(pref);
    try {
      const res = await API.put('/api/subscription/self/preference', {
        billing_preference: pref,
      });
      if (res.data?.success) {
        showSuccess(t('Updated successfully'));
        const normalizedPref =
          res.data?.data?.billing_preference || pref || previousPref;
        setBillingPreference(normalizedPref);
      } else {
        showError(res.data?.message || t('Update failed'));
        setBillingPreference(previousPref);
      }
    } catch (e) {
      showError(t('Request failed'));
      setBillingPreference(previousPref);
    }
  };

  const getTopupInfo = async () => {
    try {
      const res = await API.get('/api/user/topup/info');
      const { data, success } = res.data;
      if (!success) {
        console.error('failed to get topup info', data);
        return;
      }

      setTopupInfo({
        amount_options: data.amount_options || [],
        discount: data.discount || {},
      });

      let payMethods = data.pay_methods || [];
      if (typeof payMethods === 'string') {
        payMethods = JSON.parse(payMethods);
      }
      if (payMethods && payMethods.length > 0) {
        payMethods = payMethods
          .filter((method) => method.name && method.type)
          .map((method) => {
            const normalizedMinTopup = Number(method.min_topup);
            method.min_topup = Number.isFinite(normalizedMinTopup)
              ? normalizedMinTopup
              : 0;

            if (
              method.type === 'stripe' &&
              (!method.min_topup || method.min_topup <= 0)
            ) {
              const stripeMin = Number(data.stripe_min_topup);
              if (Number.isFinite(stripeMin)) {
                method.min_topup = stripeMin;
              }
            }

            if (!method.color) {
              if (method.type === 'alipay') {
                method.color = 'rgba(var(--semi-blue-5), 1)';
              } else if (method.type === 'wxpay' || method.type === 'wechat') {
                method.color = 'rgba(var(--semi-green-5), 1)';
              } else if (method.type === 'stripe') {
                method.color = 'rgba(var(--semi-purple-5), 1)';
              } else {
                method.color = 'rgba(var(--semi-primary-5), 1)';
              }
            }
            return method;
          });
      } else {
        payMethods = [];
      }

      setPayMethods(payMethods);
      const enableStripeTopUp = data.enable_stripe_topup || false;
      const enableWeChatTopUp = data.enable_wechat_topup || false;
      const enableAlipayTopUp = data.enable_alipay_topup || false;
      const enableOnlineTopUp = data.enable_online_topup || false;
      const enableCreemTopUp = data.enable_creem_topup || false;
      const minTopUpValue = enableOnlineTopUp || enableWeChatTopUp || enableAlipayTopUp
        ? data.min_topup
        : enableStripeTopUp
          ? data.stripe_min_topup
          : 1;

      setEnableOnlineTopUp(enableOnlineTopUp);
      setEnableStripeTopUp(enableStripeTopUp);
      setEnableWeChatTopUp(enableWeChatTopUp);
      setEnableAlipayTopUp(enableAlipayTopUp);
      setEnableCreemTopUp(enableCreemTopUp);
      setMinTopUp(minTopUpValue);
      setTopUpCount(minTopUpValue);

      try {
        const products = JSON.parse(data.creem_products || '[]');
        setCreemProducts(products);
      } catch (e) {
        setCreemProducts([]);
      }

      if (topupInfo.amount_options.length === 0) {
        setPresetAmounts(generatePresetAmounts(minTopUpValue));
      }

      getAmount(minTopUpValue);

      if (data.amount_options && data.amount_options.length > 0) {
        const customPresets = data.amount_options.map((amount) => ({
          value: amount,
          discount: data.discount[amount] || 1.0,
        }));
        setPresetAmounts(customPresets);
      }
    } catch (error) {
      console.error('failed to get topup info', error);
    }
  };

  const getAffLink = async () => {
    const res = await API.get('/api/user/aff');
    const { success, message, data } = res.data;
    if (success) {
      let link = `${window.location.origin}/register?aff=${data}`;
      setAffLink(link);
    } else {
      showError(message);
    }
  };

  const transfer = async () => {
    if (transferAmount < getQuotaPerUnit()) {
      showError(t('Transfer amount must be at least') + ' ' + renderQuota(getQuotaPerUnit()));
      return;
    }
    const res = await API.post(`/api/user/aff_transfer`, {
      quota: transferAmount,
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(message);
      setOpenTransfer(false);
      getUserQuota().then();
    } else {
      showError(message);
    }
  };

  const handleAffLinkClick = async () => {
    await copy(affLink);
    showSuccess(t('Invitation link copied'));
  };

  useEffect(() => {
    // Keep user quota data up to date on first load.
    getUserQuota().then();
    setTransferAmount(getQuotaPerUnit());
  }, []);

  useEffect(() => {
    if (affFetchedRef.current) return;
    affFetchedRef.current = true;
    getAffLink().then();
  }, []);

  useEffect(() => {
    getTopupInfo().then();
    getSubscriptionPlans().then();
    getSubscriptionSelf().then();
  }, []);

  useEffect(() => {
    if (statusState?.status) {
      // const minTopUpValue = statusState.status.min_topup || 1;
      // setMinTopUp(minTopUpValue);
      // setTopUpCount(minTopUpValue);
      setTopUpLink(statusState.status.top_up_link || '');
      setPriceRatio(statusState.status.price || 1);

      setStatusLoading(false);
    }
  }, [statusState?.status]);

  const renderAmount = () => {
    return amount + ' ' + t('CNY');
  };

  const getAmount = async (value) => {
    if (value === undefined) {
      value = topUpCount;
    }
    setAmountLoading(true);
    try {
      const res = await API.post('/api/user/amount', {
        amount: parseFloat(value),
      });
      if (res !== undefined) {
        const { message, data } = res.data;
        if (message === 'success') {
          setAmount(parseFloat(data));
        } else {
          setAmount(0);
          Toast.error({ content: t('Error') + ': ' + data, id: 'getAmount' });
        }
      } else {
        showError(res);
      }
    } catch (err) {
      console.log(err);
    }
    setAmountLoading(false);
  };

  const getStripeAmount = async (value) => {
    if (value === undefined) {
      value = topUpCount;
    }
    setAmountLoading(true);
    try {
      const res = await API.post('/api/user/stripe/amount', {
        amount: parseFloat(value),
      });
      if (res !== undefined) {
        const { message, data } = res.data;
        if (message === 'success') {
          setAmount(parseFloat(data));
        } else {
          setAmount(0);
          Toast.error({ content: t('Error') + ': ' + data, id: 'getAmount' });
        }
      } else {
        showError(res);
      }
    } catch (err) {
      console.log(err);
    } finally {
      setAmountLoading(false);
    }
  };

  const handleCancel = () => {
    setOpen(false);
  };

  const handleTransferCancel = () => {
    setOpenTransfer(false);
  };

  const handleOpenHistory = () => {
    setOpenHistory(true);
  };

  const handleHistoryCancel = () => {
    setOpenHistory(false);
  };

  const handleCreemCancel = () => {
    setCreemOpen(false);
    setSelectedCreemProduct(null);
  };
  const handleWeChatPayCancel = () => {
    setWechatPayOpen(false);
  };

  const queryTopupTradeStatus = async (tradeNo) => {
    if (!tradeNo) return '';
    const res = await API.get(
      `/api/user/topup/self?p=1&page_size=1&keyword=${encodeURIComponent(tradeNo)}`,
    );
    if (!res?.data?.success) return '';
    const items = res.data?.data?.items || [];
    const target = items.find((item) => item?.trade_no === tradeNo);
    return target?.status || '';
  };

  useEffect(() => {
    const clearPolling = () => {
      if (wechatPayPollingRef.current) {
        clearInterval(wechatPayPollingRef.current);
        wechatPayPollingRef.current = null;
      }
    };

    if (!wechatPayOpen || !wechatPayTradeNo) {
      clearPolling();
      return () => clearPolling();
    }

    let stopped = false;
    let timeoutRef = null;
    const pollOnce = async () => {
      if (stopped) return;
      try {
        const status = await queryTopupTradeStatus(wechatPayTradeNo);
        if (status === 'success') {
          clearPolling();
          if (timeoutRef) {
            clearTimeout(timeoutRef);
            timeoutRef = null;
          }
          if (stopped) return;
          setWechatPayOpen(false);
          showSuccess(t('Payment successful'));
          getUserQuota().then();
          getTopupInfo().then();
          return;
        }
        if (
          status === 'unpaid' ||
          status === 'failed' ||
          status === 'expired'
        ) {
          clearPolling();
          if (timeoutRef) {
            clearTimeout(timeoutRef);
            timeoutRef = null;
          }
          if (stopped) return;
          setWechatPayOpen(false);
          showError(t(status === 'unpaid' ? '未支付' : '支付失败'));
        }
      } catch (e) {
        // ignore polling errors
      }
    };

    pollOnce();
    wechatPayPollingRef.current = setInterval(pollOnce, 2000);
    timeoutRef = setTimeout(() => {
      clearPolling();
      if (stopped) return;
      showInfo(t('Payment status polling timed out, please refresh manually.'));
    }, WECHAT_PAY_POLLING_TIMEOUT_MS);

    return () => {
      stopped = true;
      if (timeoutRef) {
        clearTimeout(timeoutRef);
      }
      clearPolling();
    };
  }, [wechatPayOpen, wechatPayTradeNo, t]);

  const selectPresetAmount = (preset) => {
    setTopUpCount(preset.value);
    setSelectedPreset(preset.value);

    // Calculate display amount after optional discount.
    const discount = preset.discount || topupInfo.discount[preset.value] || 1.0;
    const discountedAmount = preset.value * priceRatio * discount;
    setAmount(discountedAmount);
  };

  const formatLargeNumber = (num) => {
    return num.toString();
  };

  // Generate fallback preset amount list.
  const generatePresetAmounts = (minAmount) => {
    const multipliers = [1, 5, 10, 30, 50, 100, 300, 500];
    return multipliers.map((multiplier) => ({
      value: minAmount * multiplier,
    }));
  };

  return (
    <div className='w-full max-w-7xl mx-auto relative min-h-screen lg:min-h-0 mt-[60px] px-2'>
      <TransferModal
        t={t}
        openTransfer={openTransfer}
        transfer={transfer}
        handleTransferCancel={handleTransferCancel}
        userState={userState}
        renderQuota={renderQuota}
        getQuotaPerUnit={getQuotaPerUnit}
        transferAmount={transferAmount}
        setTransferAmount={setTransferAmount}
      />

      <PaymentConfirmModal
        t={t}
        open={open}
        onlineTopUp={onlineTopUp}
        handleCancel={handleCancel}
        confirmLoading={confirmLoading}
        topUpCount={topUpCount}
        renderQuotaWithAmount={renderQuotaWithAmount}
        amountLoading={amountLoading}
        renderAmount={renderAmount}
        payWay={payWay}
        payMethods={payMethods}
        amountNumber={amount}
        discountRate={topupInfo?.discount?.[topUpCount] || 1.0}
      />

      <TopupHistoryModal
        visible={openHistory}
        onCancel={handleHistoryCancel}
        t={t}
      />

      <Modal
        title={t('Confirm top-up?')}
        visible={creemOpen}
        onOk={onlineCreemTopUp}
        onCancel={handleCreemCancel}
        maskClosable={false}
        size='small'
        centered
        confirmLoading={confirmLoading}
      >
        {selectedCreemProduct && (
          <>
            <p>
              {t('Product name')}：{selectedCreemProduct.name}
            </p>
            <p>
              {t('Price')}：{selectedCreemProduct.currency === 'EUR' ? 'EUR ' : '$'}
              {selectedCreemProduct.price}
            </p>
            <p>
              {t('Top-up quota')}：{selectedCreemProduct.quota}
            </p>
            <p>{t('Confirm top-up?')}</p>
          </>
        )}
      </Modal>

      <Modal
        title={null}
        visible={wechatPayOpen}
        onCancel={handleWeChatPayCancel}
        footer={null}
        centered
        size='small'
        bodyStyle={{ padding: 0 }}
      >
        <div className='flex flex-col items-center py-8 px-6'>
          {/* WeChat Header */}
          <div className='flex items-center gap-3 mb-6'>
            <div className='w-10 h-10 rounded-full bg-gradient-to-br from-green-400 to-green-600 flex items-center justify-center shadow-lg'>
              <WeChatIcon />
            </div>
            <div>
              <h3 className='text-lg font-semibold text-gray-800'>{t('WeChat Pay')}</h3>
              <p className='text-xs text-gray-500'>{t('Scan with WeChat to complete payment')}</p>
            </div>
          </div>

          {/* QR Code Container */}
          <div className='relative mb-6'>
            <div className='absolute inset-0 bg-gradient-to-br from-green-50 to-emerald-50 rounded-2xl blur-xl opacity-60'></div>
            <div className='relative bg-white p-6 rounded-2xl shadow-xl border-2 border-green-100'>
              {wechatPayCodeUrl ? (
                <QRCodeSVG
                  value={wechatPayCodeUrl}
                  size={240}
                  level="H"
                  includeMargin={true}
                />
              ) : (
                <div className='w-60 h-60 flex items-center justify-center'>
                  <div className='animate-spin rounded-full h-12 w-12 border-b-2 border-green-500'></div>
                </div>
              )}
            </div>
          </div>

          {/* Order Info */}
          {wechatPayTradeNo && (
            <div className='w-full bg-gray-50 rounded-lg p-3 mb-4'>
              <p className='text-xs text-gray-500 text-center'>
                {t('Order ID')}: <span className='font-mono text-gray-700'>{wechatPayTradeNo}</span>
              </p>
            </div>
          )}

          {/* Instructions */}
          <div className='flex items-start gap-2 text-xs text-gray-600 bg-green-50 rounded-lg p-3 w-full'>
            <svg className='w-4 h-4 text-green-600 mt-0.5 flex-shrink-0' fill='currentColor' viewBox='0 0 20 20'>
              <path fillRule='evenodd' d='M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z' clipRule='evenodd' />
            </svg>
            <span>{t('Open WeChat and scan the QR code to pay. The page will automatically update after successful payment.')}</span>
          </div>
        </div>
      </Modal>

      <div className='grid grid-cols-1 lg:grid-cols-2 gap-6'>
        <RechargeCard
          t={t}
          enableOnlineTopUp={enableOnlineTopUp}
          enableStripeTopUp={enableStripeTopUp}
          enableWeChatTopUp={enableWeChatTopUp}
          enableAlipayTopUp={enableAlipayTopUp}
          enableCreemTopUp={enableCreemTopUp}
          creemProducts={creemProducts}
          creemPreTopUp={creemPreTopUp}
          presetAmounts={presetAmounts}
          selectedPreset={selectedPreset}
          selectPresetAmount={selectPresetAmount}
          formatLargeNumber={formatLargeNumber}
          priceRatio={priceRatio}
          topUpCount={topUpCount}
          minTopUp={minTopUp}
          renderQuotaWithAmount={renderQuotaWithAmount}
          getAmount={getAmount}
          setTopUpCount={setTopUpCount}
          setSelectedPreset={setSelectedPreset}
          renderAmount={renderAmount}
          amountLoading={amountLoading}
          payMethods={payMethods}
          preTopUp={preTopUp}
          paymentLoading={paymentLoading}
          payWay={payWay}
          redemptionCode={redemptionCode}
          setRedemptionCode={setRedemptionCode}
          topUp={topUp}
          isSubmitting={isSubmitting}
          topUpLink={topUpLink}
          openTopUpLink={openTopUpLink}
          userState={userState}
          renderQuota={renderQuota}
          statusLoading={statusLoading}
          topupInfo={topupInfo}
          onOpenHistory={handleOpenHistory}
          subscriptionLoading={subscriptionLoading}
          subscriptionPlans={subscriptionPlans}
          billingPreference={billingPreference}
          onChangeBillingPreference={updateBillingPreference}
          activeSubscriptions={activeSubscriptions}
          allSubscriptions={allSubscriptions}
          reloadSubscriptionSelf={getSubscriptionSelf}
        />
        <InvitationCard
          t={t}
          userState={userState}
          renderQuota={renderQuota}
          setOpenTransfer={setOpenTransfer}
          affLink={affLink}
          handleAffLinkClick={handleAffLinkClick}
        />
      </div>
    </div>
  );
};

export default TopUp;


