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
  const [statusLoading, setStatusLoading] = useState(true);

  // Creem 闂傚倸鍊搁崐鎼佸磹閻戣姤鍤勯柛顐ｆ礀绾惧潡鏌ｉ姀銏╃劸闁汇倗鍋撶换婵嬫濞戝崬鍓梺閫炲苯澧剧紒鐘虫尭閻ｉ攱绺界粙璇俱劑鏌曟竟顖氭噹楠炩偓闂備浇宕甸崰鎰垝瀹ュ憘娑㈠礃椤旀儳绁﹂梺鍛婂姦閸犳牠鎮″☉銏＄厱婵炴垵宕弸娑欑箾閸喓绠樼紒杈ㄥ浮瀹曟粏顦叉い锝呫偢閺屻劌顫濋?
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

  // 闂傚倸鍊搁崐鎼佸磹閹间礁纾瑰瀣椤愪粙鏌ㄩ悢鍝勑㈤柛灞诲姂閺岀喖姊荤€靛壊妲紒鐐劤椤兘寮婚悢鐓庣鐟滃繒鏁☉銏＄厱閻庯絽澧庣粔顕€鏌＄仦鍓ф创濠碉紕鍏橀、娆撴偂鎼存ɑ瀚藉┑鐘殿暯濡插懘宕戦崨鏉戝瀭闁告挷鐒﹀畷鍙夌箾閹存瑥鐏╃紒鐙欏洦鐓欓柟纰卞幖楠炴鏌嶇拠鑼ⅵ闁诡喗顨堥幉鎾礋椤掑偆妲版俊鐐€戦崝宀勫箠濮椻偓楠炲啯銈ｉ崘鈺佲偓濠氭煢濡警妲洪柛濠勫仜椤啴濡舵惔鈥斥拻闂侀潻缍嗛崹宕囧垝婵犳艾绠婚悹鍥у级閻?
  const [affLink, setAffLink] = useState('');
  const [openTransfer, setOpenTransfer] = useState(false);
  const [transferAmount, setTransferAmount] = useState(0);

  // 闂傚倸鍊搁崐宄懊归崶褏鏆﹂柛顭戝亝閸欏繘鏌ｉ姀銏╃劸缂佲偓婢跺本鍠愰煫鍥ㄦ礀閸ㄦ繂鈹戦悩瀹犲缂佺媴绲剧换婵嬫濞戞艾顤€濠电偛鐗撶粻鏍ь潖閾忓湱纾兼俊顖濇娴犵厧顪冮妶鍐ㄧ仼婵☆垰篓l闂傚倸鍊搁崐鎼佸磹閻戣姤鍤勯柛鎾茬閸ㄦ繃銇勯弽顐汗闁逞屽墾缁犳挸鐣锋總绋课ㄩ柕澶涢檮琚ｉ梻鍌欑閹碱偆绮欐笟鈧畷銏＄鐎ｎ亞鏌?
  const [openHistory, setOpenHistory] = useState(false);

  // 闂傚倸鍊搁崐宄懊归崶褏鏆﹂柛顭戝亝閸欏繘鏌熼幆鏉啃撻柍閿嬫⒒閳ь剙绠嶉崕閬嵥囬鐐插瀭闁稿瞼鍋為悡銏′繆椤栨粌鐨戠紒杈ㄥ哺閺屻劌鈹戦崱鈺傂︾紓浣插亾閻庯綆鍋佹禍婊堟煛瀹ュ啫濡块柍钘夘樀閺屾盯骞欓崘銊︾亾缂備浇椴搁幐濠氬箯閸涙潙绀堥柟缁樺笒婢瑰牓姊绘担绛嬪殭缂佺粯鍨归幑銏ゅ幢濞戞?
  const [subscriptionPlans, setSubscriptionPlans] = useState([]);
  const [subscriptionLoading, setSubscriptionLoading] = useState(true);
  const [billingPreference, setBillingPreference] =
    useState('subscription_first');
  const [activeSubscriptions, setActiveSubscriptions] = useState([]);
  const [allSubscriptions, setAllSubscriptions] = useState([]);

  // Preset top-up amount options.
  const [presetAmounts, setPresetAmounts] = useState([]);
  const [selectedPreset, setSelectedPreset] = useState(null);

  // 闂傚倸鍊搁崐鎼佸磹閻戣姤鍤勯柛顐ｆ磵閳ь剨绠撳畷濂稿閳ュ啿绨ラ梻浣告贡婢ф顭垮Ο鑲╀笉闁绘顕х粻瑙勭箾閿濆骸澧┑鈥炽偢閺岋綁骞掗幋鐘辩驳闂侀潧娲ょ€氫即鐛幒妤€骞㈡俊鐐村劤椤ユ岸姊婚崒娆戠獢婵炰匠鍥ㄥ亱濠电姴娲ょ粻鏍煕鐏炵偓鐨戦柡鍡畵閺屾洝绠涚€ｎ亖鍋撻弴鐐寸函闂傚倷鐒﹂幃鍫曞磿椤曗偓瀵彃鈹戠€ｎ偅娅栭梺缁樺姇濡﹤銆掓繝姘厪闁割偅绻冮ˉ鐐淬亜閵夈儲顥為柕鍥у瀵剟宕犻垾鍐差潬闂備胶鎳撶粻宥夊垂閽樺鏆﹂柛妤冨亹濡插牊绻涢崱妯哄妞?
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
        showError(t('WeChat Pay is not enabled by admin'));
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
    setConfirmLoading(true);
    try {
      let res;
      if (payWay === 'stripe') {
        res = await API.post('/api/user/stripe/pay', {
          amount: parseInt(topUpCount),
          payment_method: 'stripe',
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
            window.open(data.pay_link, '_blank');
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
    // 闂?Stripe 濠电姷鏁告慨鐑藉极閹间礁纾块柟瀵稿Т缁躲倝鏌﹀Ο渚＆婵炲樊浜濋弲婊堟煟閹伴潧澧幖鏉戯躬濮婃椽宕ㄦ繝鍌毿曟繛瀛樼矋閻楃姴鐣烽幋锕€绠婚悹鍥ㄥ絻閻庮厼顪冮妶鍡楀Ё缂佸弶妞介獮蹇涘捶椤撶姷锛濇繛杈剧到婢瑰﹪宕曡箛鏂讳簻闁挎梻鍋撻弳顒傗偓瑙勬礃濡炰粙宕洪埀顒併亜閹哄秹妾峰ù婊勭矒閺岀喖鎮滃Ο铏逛淮濡炪倕娴氭禍顏堝蓟濞戙垹围闁告粌鍟抽崥顐︽⒑鐠団€虫灍妞ゃ劌锕よ灋闁告劑鍔夊Σ鍫熺箾閸℃绠叉繛鍫燂耿濮婄粯鎷呮搴濊缂傚倸绉抽悞锔剧矉瀹ュ閱囬柡鍥╁仧閻ｅ搫鈹戞幊閸婃洟骞婃惔銊ョ闁煎摜鍋ｆ禍婊堟煙閹规劖纭惧ù鐘冲浮閺岀喖寮堕幋婵囆╅柧缁樼墵閺岋絽顫滈埀顒€顭囪缁傛帒顭ㄩ崟顓狀啎閻庣懓澹婇崰鏍嵁閺嶃劊浜滈柡鍥朵簽缁夘喗銇勯姀鈽嗘疁鐎规洜鍠栭、妤呭焵椤掍胶顩锋い鎾卞灪閳锋垿鎮峰▎蹇擃伌闁哥喎绻橀弻娑㈡偐閸愭彃鎽甸梺?
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

  // 闂傚倸鍊搁崐鎼佸磹妞嬪海鐭嗗〒姘ｅ亾鐎规洏鍎抽埀顒婄秵閸犳牜澹曢崸妤佺厵闁诡垳澧楅ˉ澶愬箹閺夋埊韬柡灞诲€濋幊婵嬪箥椤旇偐澧┑鐐茬摠缁瞼绱炴繝鍥ц摕婵炴垯鍨瑰敮濡炪倖姊婚崢褔锝為埡鍐＝濞达絾褰冩禍楣冩⒑缁嬭法鐏遍柛瀣仱閹繝寮撮悢铏诡啎闂佺懓鐡ㄧ换鍌炴嚋鐟欏嫷娈介柣鎰綑婵秹鏌＄仦鍓с€掗柍褜鍓ㄧ紞鍡涘磻閸涱垯鐒婇柟娈垮枤绾惧ジ鏌ц箛姘兼綈婵炲懏娲滅槐鎺楊敊閻ｅ本鍣伴悗瑙勬礀缂嶅﹪銆佸▎鎴炲磯闁靛绲芥禒锔剧磽閸屾艾鈧悂宕愰幖浣哥９闁归棿绀佺壕褰掓煙闂傚顦︾紒鎰殕閹便劌螣閹稿海銆愮紓浣瑰姈椤ㄥ﹪寮婚敓鐘茬倞闁宠桨鐒﹂悘渚€姊哄畷鍥ㄥ殌缂佸鎸抽崺鐐哄箣閿旇棄浜归梺鍛婄懃椤︿即骞冨▎鎴犵＝?
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
      const enableOnlineTopUp = data.enable_online_topup || false;
      const enableCreemTopUp = data.enable_creem_topup || false;
      const minTopUpValue = enableOnlineTopUp || enableWeChatTopUp
        ? data.min_topup
        : enableStripeTopUp
          ? data.stripe_min_topup
          : 1;

      setEnableOnlineTopUp(enableOnlineTopUp);
      setEnableStripeTopUp(enableStripeTopUp);
      setEnableWeChatTopUp(enableWeChatTopUp);
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

  // 闂傚倸鍊搁崐鎼佸磹妞嬪海鐭嗗〒姘ｅ亾鐎规洏鍎抽埀顒婄秵閸犳牜澹曢崸妤佺厵闁诡垳澧楅ˉ澶愬箹閺夋埊韬柡灞诲€濋幊婵嬪箥椤旇偐澧┑鐐茬摠缁瞼绱炴繝鍥ц摕婵炴垯鍨瑰敮濡炪倖姊婚崢褔锝為鍫熲拺缂備焦锚缁楀倻绱掗鐣屾噰鐎殿喛顕ч濂稿醇椤愶綆鈧洭姊绘担鍛婂暈闁圭顭烽幆鍕敍閻愰潧绁﹂棅顐㈡处缁嬫帡宕戦幇顔剧＝濞达絽顫栭鍫濈劦妞ゆ巻鍋撴い顓犲厴瀵濡舵径濠勭暢闂佸湱鍎ら崹鍨叏閺囥垺鈷戦柟鑲╁仜閳ь剚鐗犲畷褰掑锤濡も偓閽?
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

  // 闂傚倸鍊搁崐鎼佸磹妞嬪海鐭嗗〒姘ｅ亾妤犵偛顦甸弫宥夊礋椤掍焦顔囨繝寰锋澘鈧洟宕鐐叉辈婵犲﹤鐗婇悡鐔兼煛閸愩劍绁╅柛鐔风箳閹叉悂鎮ч崼婵堢懆濡ょ姷鍋戦崹铏规崲濞戙垹绠ｉ柣鎰仛閸ｎ噣姊洪崨濠冣拻闁稿繑锚椤繐煤椤忓懐鍔甸梺缁樺姌鐏忣亞鈧碍婢橀…鑳槺闁告濞婂濠氭偄閸撳弶效闁硅偐琛ュ褔寮搁崨瀛樷拺閻犲洠鈧櫕鐏嶇紓渚囧枛濞寸兘宕氶幒妤佸仺缁剧増锚娴滈箖鏌ㄥ┑鍡涱€楀ù婊勭箘缁辨帗锛愬┑鍡楃睄濠?
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

  // 濠电姷鏁告慨鐑藉极閸涘﹥鍙忓ù鍏兼綑閸ㄥ倿鏌ｉ幘宕囧哺闁哄鐗楃换娑㈠箣閻愬娈ら梺娲诲幗閹瑰洭寮婚悢铏圭＜闁靛繒濮甸悘鍫ユ⒑閸濆嫬顏ラ柛搴″级缁岃鲸绻濋崶顬囨煕濞戝崬鏋涙繛鍛€濆铏圭磼濡櫣鐟愮紓渚囧枟閻熲晛顕ｇ拠宸悑闁割偒鍋呴鍥⒒娴ｅ憡鍟為柟绋款煼閹嫰顢涢悙闈涚ウ闂婎偄娲︾粙鎺楀磻閹邦喚纾藉ù锝咁潠椤忓牆鐒垫い鎺嗗亾妞ゎ厾鍏樺濠氬Χ婢跺﹦鐣抽梺鍦劋閸ㄥ灚鎱ㄩ弴銏♀拺闁硅偐鍋涢埀顒佺墵瀹曞綊宕稿Δ鈧拑?
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

  // 闂傚倸鍊搁崐鎼佸磹閹间礁纾瑰瀣椤愪粙鏌ㄩ悢鍝勑㈢痪鎯ь煼閺屾盯寮撮妸銉р偓顒勬煕閵夘喖澧紒鐘劜閵囧嫰寮崒娑樻畬婵炲瓨绮庨崑鎾诲箞閵娿儙鐔虹矙閸喖顫撳┑鐘殿暯閳ь剝灏欓惌娆撴煛鐏炶濡奸柍钘夘槸閳诲氦绠涢幙鍐╃稈闂佽姘﹂～澶娒哄鍫濈獥闁哄稁鍘归埀顑跨椤繄鎹勯崫鍕崺婵＄偑鍊栭幐楣冨窗閹扮増鍋熸い鎺戝閳锋帡鏌涚仦鍓ф噯闁稿繐鏈妵鍕敇閻愰潧鈪遍梺鐟扮畭閸ㄤ粙鐛崶顒€绾ч悹鎭掑妿閺嬪啴姊洪悷鏉挎倯闁伙綆浜畷婵嗙暆閸曨剙鈧潡鏌涢…鎴濅簴濞存粍绮撻弻鐔煎传閸曨剦妫炴繛瀛樼矒缁犳牕顫忓ú顏咁棃婵炴垶鑹鹃·鈧梻浣筋嚃閸犳牠宕愭禒瀣剹濡わ絽鍟埛鎴犵磼鐎ｎ偄顕滄繝鈧幍顔剧＜閻庯綆鍋勫ù顔锯偓?
  const selectPresetAmount = (preset) => {
    setTopUpCount(preset.value);
    setSelectedPreset(preset.value);

    // Calculate display amount after optional discount.
    const discount = preset.discount || topupInfo.discount[preset.value] || 1.0;
    const discountedAmount = preset.value * priceRatio * discount;
    setAmount(discountedAmount);
  };

  // 闂傚倸鍊搁崐鎼佸磹妞嬪海鐭嗗〒姘ｅ亾妤犵偞鐗犻、鏇㈠Χ閸モ晝鍘犻梻浣稿閸嬪懎煤閺嶎厼纾奸柕濞у嫬鏋戦棅顐㈡处閹峰綊鏁愭径濠勭杸闂佺粯顨呴悧濠傗枍閵忋倖鈷戠紓浣癸供閻掗箖鎮樿箛鏃傛噰閽樻繈姊洪鈧粔鐢稿煕閹达附鐓曟繝闈涙椤忣剟鏌ｈ缁€渚€婀侀梺鎸庣箓椤︿即寮柆宥嗙厵闁谎冩憸缁夘喗鎱ㄦ繝鍕笡闁瑰嘲鎳橀幊鐐哄Ψ閿濆倸浜鹃柛鎰靛枟閻撶喖鏌熼搹鐟颁户闁伙綀椴搁妵鍕敇閻愬吀铏庡銈庡亝缁捇宕洪埀顒併亜閹烘垵顏╃紒鐘崇墪铻栭柨鏃傜摂閸庛儲淇婇姘伃闁诡喗顨呴埢鎾诲垂椤旂晫浜為梻浣告啞椤ㄥ懘宕崸妤佸仼鐎瑰嫰鍋婇悡銉╂煕閳锯偓閺呮粍鏅ュ┑?
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
      {/* 闂傚倸鍊搁崐鎼佸磹妞嬪海鐭嗗〒姘ｅ亾妤犵偛顦甸弫宥夊礋椤掍焦顔囨繝寰锋澘鈧洟宕鐐叉辈婵犲﹤鐗婇悡鐔兼煛閸愩劍绁╅柛鐔风箳閹叉悂鎮ч崼婵堢懆濡ょ姷鍋戦崹鐑樼┍婵犲洦鍊锋い蹇撳娴煎嫰姊洪崫銉バｉ柣妤冨Т閻ｅ嘲煤椤忓嫮顦板銈嗙墬濮樸劑藝閵娾晜鈷戦柟绋挎捣缁犳捇鏌熼崘鑼ｇ紒鏃傚枛瀵挳鎮㈤搹璇″晭闂佸搫顦悧鍡樻櫠閻ｅ瞼鐭撴い鎺嶉檷娴滄粓鏌￠崶鈺佹灁濠⒀呮暩閳?*/}
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

      {/* 闂傚倸鍊搁崐鎼佸磹閻戣姤鍤勯柛顐ｆ磵閳ь剨绠撳畷濂稿閳ュ啿绨ラ梻浣告贡婢ф顭垮Ο鑲╀笉闁绘顕х粻瑙勭箾閿濆骸澧┑鈥炽偢閺岋綁骞掗幋鐘辩驳闂侀潧娲ょ€氫即鐛幒妤€绠ｆ繝闈涙处椤斿嫬鈹戦悙鑼憼缂侇喖鐬肩槐鐐寸節閸パ嗘憰閻庡箍鍎遍ˇ顖氭暜闂備焦瀵уΛ浣哥暦閻㈢绀夐柣鎴ｅГ閳锋垿鏌ｉ悢鍛婄凡婵¤尙绮妵鍕箣濠垫劖效婵烇絽娲ら敃顏勭暦婵傜鍗抽柣鎰暩瀹曞爼姊绘担鐟邦嚋缂佽鍊婚埀顒佸嚬閸犳氨鍒掗敐澶婄睄闁逞屽墴閵嗗啴濡烽埡鍌氣偓鐑芥煃鏉炵増顦峰瑙勬礃缁绘繈鎮介棃娑楀摋闂佽妞挎禍鐐垫閻愬搫鐒垫い鎺嶉檷娴滄粓鏌￠崶鈺佹灁濠⒀呮暩閳?*/}
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

      {/* 闂傚倸鍊搁崐鎼佸磹閻戣姤鍤勯柛顐ｆ磵閳ь剨绠撳畷濂稿閳ュ啿绨ラ梻浣告贡婢ф顭垮Ο鑲╀笉闁绘顕х粻瑙勭箾閿濆骸澧┑鈥炽偢閺岋綁骞掗幋鐘辩驳闂侀潧娲ょ€氫即鐛幒妤€绠ｆ繝闈涙处椤斿嫭淇婇悙顏勨偓鎴﹀磿鏉堚晝鐭撻柟缁㈠枓閳ь剨濡囬幑鍕Ω閿曗偓绾绢垶姊洪崨濠勭畵閻庢岸鏀辩€靛ジ宕橀钘夆偓鐢告偡濞嗗繐顏紒宀冩硶缁辨帞绱掑Ο鑲╃暤濡炪倖娲╃紞渚€鐛鈧、娆撴寠婢跺鐩庨梻浣筋嚙缁绘帡宕戦悩璇茬；闁归偊鍠氶悳濠氭煛閸愶絽浜惧銈冨妸閸庣敻骞冨▎鎾崇厸濞达絽鍢查ˉ姘攽閻樺灚鏆╅柛瀣█楠炴捇顢旈崱娆戭槸闂侀€炲苯澧柕鍥у瀵挳鎮㈤崫鍕ㄦ嫲闁?*/}
      <TopupHistoryModal
        visible={openHistory}
        onCancel={handleHistoryCancel}
        t={t}
      />

      {/* Creem 闂傚倸鍊搁崐鎼佸磹閻戣姤鍤勯柛顐ｆ磵閳ь剨绠撳畷濂稿閳ュ啿绨ラ梻浣告贡婢ф顭垮Ο鑲╀笉闁绘顕х粻瑙勭箾閿濆骸澧┑鈥炽偢閺岋綁骞掗幋鐘辩驳闂侀潧娲ょ€氫即鐛幒妤€绠ｆ繝闈涙处椤斿嫬鈹戦悙鑼憼缂侇喖鐬肩槐鐐寸節閸パ嗘憰閻庡箍鍎遍ˇ顖氭暜闂備焦瀵уΛ浣哥暦閻㈢绀夐柣鎴ｅГ閳锋垿鏌ｉ悢鍛婄凡婵¤尙绮妵鍕箣濠垫劖效婵烇絽娲ら敃顏勭暦婵傜鍗抽柣鎰暩瀹曞爼姊绘担鐟邦嚋缂佽鍊婚埀顒佸嚬閸犳氨鍒掗敐澶婄睄闁逞屽墴閵嗗啴濡烽埡鍌氣偓鐑芥煃鏉炵増顦峰瑙勬礃缁绘繈鎮介棃娑楀摋闂佽妞挎禍鐐垫閻愬搫鐒垫い鎺嶉檷娴滄粓鏌￠崶鈺佹灁濠⒀呮暩閳?*/}
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

      {/* 濠电姷鏁告慨鐑藉极閹间礁纾婚柣鎰惈閸ㄥ倿鏌涢锝嗙缂佺姵褰冮妴鎺戭潩閿濆懍澹曟俊鐐€戦崹鐑樼┍濞差€洨鈧潧鎽滅壕鍏笺亜閺冨浂娼愭繛鍛閵囧嫰鏁傞幆褜鏆梺璇″灡閺屻劑鍩為幋锕€鐐婄憸宥夘敁韫囨稒鈷掗柛灞剧懅缁愭棃鏌嶈閸撴盯宕戝☉銏″殣妞ゆ牗绋掑▍鐘绘倵閻㈢數銆婇柛瀣尵閹叉挳宕熼鍌ゆФ濠电姷鏁搁崑鎰板磻閹剧粯鍊甸悷娆忓婢跺嫰鏌涚€ｎ亷宸ラ柣锝囧厴閹垻鍠婃潏銊︽珝闂備胶绮崝妯间焊濞嗘搩鏁?*/}
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


