import React, { useState, useEffect, useCallback, useMemo } from 'react';
import {
  Tag,
  Input,
  Button,
  Space,
  Typography,
  Empty,
} from '@douyinfe/semi-ui';
import {
  IconSearch,
  IconRefresh,
  IconCreditCard,
} from '@douyinfe/semi-icons';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { useTranslation } from 'react-i18next';
import {
  API,
  renderQuota,
  showError,
  timestamp2string,
  getCurrencyConfig,
} from '../../helpers';
import CardPro from '../../components/common/ui/CardPro';
import CardTable from '../../components/common/ui/CardTable';
import { createCardProPagination } from '../../helpers/utils';
import { useIsMobile } from '../../hooks/common/useIsMobile';

const { Text } = Typography;

const PAYMENT_METHOD_MAP = {
  stripe: 'Stripe',
  creem: 'Creem',
  alipay: '支付宝',
  wxpay: '微信',
  redemption: '兑换码充值',
};

const AgentTopups = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const currencySymbol = getCurrencyConfig().symbol || '$';

  const [records, setRecords] = useState([]);
  const [loading, setLoading] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [total, setTotal] = useState(0);
  const [keyword, setKeyword] = useState('');

  const loadRecords = useCallback(async (page, size, search) => {
    setLoading(true);
    try {
      const params = [`p=${page}`, `page_size=${size}`];
      if (search) {
        params.push(`keyword=${encodeURIComponent(search)}`);
      }
      const res = await API.get(`/api/agent/topups?${params.join('&')}`);
      if (res.data.success) {
        const data = res.data.data;
        setRecords(data.items || []);
        setTotal(data.total || 0);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err.message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadRecords(activePage, pageSize, keyword);
  }, [activePage, pageSize, loadRecords]);

  const handleSearch = () => {
    setActivePage(1);
    loadRecords(1, pageSize, keyword);
  };

  const handleReset = () => {
    setKeyword('');
    setActivePage(1);
    loadRecords(1, pageSize, '');
  };

  const renderSource = (value) => {
    if (value === 'redemption') {
      return (
        <Tag color='orange' shape='circle'>
          {t('兑换码充值')}
        </Tag>
      );
    }
    return (
      <Tag color='green' shape='circle'>
        {t('在线充值')}
      </Tag>
    );
  };

  const renderPaymentMethod = (value) => {
    const key = PAYMENT_METHOD_MAP[value] || value || '-';
    return <span>{t(key)}</span>;
  };

  const columns = useMemo(
    () => [
      {
        title: 'ID',
        dataIndex: 'record_id',
        width: 80,
      },
      {
        title: t('时间'),
        dataIndex: 'created_at',
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('用户名'),
        dataIndex: 'username',
        render: (value) => value || '-',
      },
      {
        title: t('来源'),
        dataIndex: 'source',
        render: (value) => renderSource(value),
      },
      {
        title: t('订单号'),
        dataIndex: 'trade_no',
        render: (value) => value || '-',
      },
      {
        title: t('支付方式'),
        dataIndex: 'payment_method',
        render: (value) => renderPaymentMethod(value),
      },
      {
        title: t('充值额度'),
        dataIndex: 'quota',
        render: (value) => renderQuota(value || 0),
      },
      {
        title: t('充值'),
        dataIndex: 'money',
        render: (value) => `${currencySymbol}${Number(value || 0).toFixed(2)}`,
      },
    ],
    [t, currencySymbol],
  );

  const descriptionArea = (
    <div className='flex items-center text-blue-500'>
      <IconCreditCard className='mr-2' />
      <Text>{t('充值账单')}</Text>
    </div>
  );

  const actionsArea = (
    <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
      <div />
      <Space>
        <Input
          prefix={<IconSearch />}
          placeholder={`${t('用户名')}/${t('订单号')}`}
          value={keyword}
          onChange={(val) => setKeyword(val)}
          onEnterPress={handleSearch}
          style={{ width: 260 }}
        />
        <Button theme='solid' type='primary' onClick={handleSearch}>
          {t('查询')}
        </Button>
        <Button icon={<IconRefresh />} onClick={handleReset}>
          {t('重置')}
        </Button>
      </Space>
    </div>
  );

  return (
    <div className='mt-[60px] px-4'>
      <CardPro
        type='type1'
        descriptionArea={descriptionArea}
        actionsArea={actionsArea}
        paginationArea={createCardProPagination({
          currentPage: activePage,
          pageSize,
          total,
          onPageChange: setActivePage,
          onPageSizeChange: (size) => {
            setPageSize(size);
            setActivePage(1);
          },
          isMobile,
          t,
        })}
        t={t}
      >
        <CardTable
          columns={columns}
          dataSource={records}
          loading={loading}
          rowKey={(item) => `${item.source}-${item.record_id}`}
          scroll={isMobile ? undefined : { x: '100%' }}
          hidePagination={true}
          empty={
            <Empty
              image={
                <IllustrationNoResult style={{ width: 150, height: 150 }} />
              }
              darkModeImage={
                <IllustrationNoResultDark
                  style={{ width: 150, height: 150 }}
                />
              }
              description={t('暂无充值记录')}
              style={{ padding: 30 }}
            />
          }
          className='overflow-hidden'
          size='middle'
        />
      </CardPro>
    </div>
  );
};

export default AgentTopups;
