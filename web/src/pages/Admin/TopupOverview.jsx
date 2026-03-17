import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  DatePicker,
  Empty,
  Input,
  Select,
  Space,
  Tabs,
  TabPane,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import {
  IconCalendar,
  IconCreditCard,
  IconRefresh,
  IconSearch,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import {
  API,
  getCurrencyConfig,
  renderQuota,
  renderQuotaWithAmount,
  showError,
  timestamp2string,
} from '../../helpers';
import CardPro from '../../components/common/ui/CardPro';
import CardTable from '../../components/common/ui/CardTable';
import { createCardProPagination } from '../../helpers/utils';
import { useIsMobile } from '../../hooks/common/useIsMobile';

const { Text } = Typography;

const PAYMENT_METHOD_MAP = {
  epay: '易支付',
  stripe: 'Stripe',
  creem: 'Creem',
  alipay: '支付宝',
  wxpay: '微信',
  wechat: '微信支付',
  redemption: '兑换码充值',
};

const STATUS_COLOR_MAP = {
  success: 'green',
  pending: 'orange',
  unpaid: 'grey',
  expired: 'red',
  failed: 'red',
};

const STATUS_LABEL_MAP = {
  success: '成功',
  pending: '处理中',
  unpaid: '未支付',
  expired: '已过期',
  failed: '支付失败',
};

const getDefaultRange = () => {
  const now = new Date();
  const start = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 29);
  return [start, now];
};

const normalizeDateRange = (range) => {
  const [start, end] = range || [];
  if (!start || !end) {
    return { startTimestamp: 0, endTimestamp: 0 };
  }
  const normalizedStart = new Date(
    start.getFullYear(),
    start.getMonth(),
    start.getDate(),
    0,
    0,
    0,
    0,
  );
  const normalizedEnd = new Date(
    end.getFullYear(),
    end.getMonth(),
    end.getDate(),
    23,
    59,
    59,
    999,
  );
  return {
    startTimestamp: Math.floor(normalizedStart.getTime() / 1000),
    endTimestamp: Math.floor(normalizedEnd.getTime() / 1000),
  };
};

const AdminTopupOverview = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const currencySymbol = getCurrencyConfig().symbol || '$';

  const [overview, setOverview] = useState(null);
  const [records, setRecords] = useState([]);
  const [overviewLoading, setOverviewLoading] = useState(false);
  const [recordsLoading, setRecordsLoading] = useState(false);
  const [dateRange, setDateRange] = useState(getDefaultRange());
  const [appliedDateRange, setAppliedDateRange] = useState(getDefaultRange());
  const [keyword, setKeyword] = useState('');
  const [appliedKeyword, setAppliedKeyword] = useState('');
  const [status, setStatus] = useState('');
  const [appliedStatus, setAppliedStatus] = useState('');
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [total, setTotal] = useState(0);
  const [reloadKey, setReloadKey] = useState(0);

  const buildRangeParams = useCallback((range) => {
    const { startTimestamp, endTimestamp } = normalizeDateRange(range);
    const params = [];
    if (startTimestamp) {
      params.push(`start_timestamp=${startTimestamp}`);
    }
    if (endTimestamp) {
      params.push(`end_timestamp=${endTimestamp}`);
    }
    return params;
  }, []);

  const loadOverview = useCallback(async () => {
    setOverviewLoading(true);
    try {
      const params = buildRangeParams(appliedDateRange);
      const res = await API.get(
        `/api/user/topup/stats${params.length ? `?${params.join('&')}` : ''}`,
      );
      if (!res.data.success) {
        throw new Error(res.data.message);
      }
      setOverview(res.data.data || null);
    } catch (error) {
      showError(error.message);
    } finally {
      setOverviewLoading(false);
    }
  }, [appliedDateRange, buildRangeParams]);

  const loadRecords = useCallback(
    async (page, size) => {
      setRecordsLoading(true);
      try {
        const params = [
          `p=${page}`,
          `page_size=${size}`,
          ...buildRangeParams(appliedDateRange),
        ];
        if (appliedKeyword) {
          params.push(`keyword=${encodeURIComponent(appliedKeyword)}`);
        }
        if (appliedStatus) {
          params.push(`status=${encodeURIComponent(appliedStatus)}`);
        }
        const res = await API.get(`/api/user/topup/records?${params.join('&')}`);
        if (!res.data.success) {
          throw new Error(res.data.message);
        }
        const data = res.data.data || {};
        setRecords(data.items || []);
        setTotal(data.total || 0);
      } catch (error) {
        showError(error.message);
      } finally {
        setRecordsLoading(false);
      }
    },
    [appliedDateRange, appliedKeyword, appliedStatus, buildRangeParams],
  );

  useEffect(() => {
    loadOverview();
  }, [loadOverview, reloadKey]);

  useEffect(() => {
    loadRecords(activePage, pageSize);
  }, [activePage, pageSize, loadRecords, reloadKey]);

  const handleSearch = () => {
    const nextKeyword = keyword.trim();
    const filtersChanged =
      JSON.stringify(normalizeDateRange(appliedDateRange)) !==
        JSON.stringify(normalizeDateRange(dateRange)) ||
      appliedKeyword !== nextKeyword ||
      appliedStatus !== status ||
      activePage !== 1;

    setAppliedDateRange(dateRange);
    setAppliedKeyword(nextKeyword);
    setAppliedStatus(status);
    setActivePage(1);

    if (!filtersChanged) {
      setReloadKey((prev) => prev + 1);
    }
  };

  const handleReset = () => {
    const nextRange = getDefaultRange();
    const filtersChanged =
      JSON.stringify(normalizeDateRange(appliedDateRange)) !==
        JSON.stringify(normalizeDateRange(nextRange)) ||
      appliedKeyword !== '' ||
      appliedStatus !== '' ||
      activePage !== 1;

    setDateRange(nextRange);
    setAppliedDateRange(nextRange);
    setKeyword('');
    setAppliedKeyword('');
    setStatus('');
    setAppliedStatus('');
    setActivePage(1);

    if (!filtersChanged) {
      setReloadKey((prev) => prev + 1);
    }
  };

  const summaryCards = useMemo(() => {
    const data = overview || {};
    return [
      {
        label: t('累计充值'),
        value: `${currencySymbol}${Number(data.total_topup_amount || 0).toFixed(2)}`,
      },
      {
        label: t('累计充值笔数'),
        value: Number(data.total_topup_count || 0),
      },
      {
        label: t('累计消耗'),
        value: renderQuota(data.total_consume_quota || 0),
      },
      {
        label: t('区间充值'),
        value: `${currencySymbol}${Number(data.range_topup_amount || 0).toFixed(2)}`,
      },
      {
        label: t('区间充值笔数'),
        value: Number(data.range_topup_count || 0),
      },
      {
        label: t('区间消耗'),
        value: renderQuota(data.range_consume_quota || 0),
      },
      {
        label: t('今日充值'),
        value: `${currencySymbol}${Number(data.today_topup_amount || 0).toFixed(2)}`,
      },
      {
        label: t('今日消耗'),
        value: renderQuota(data.today_consume_quota || 0),
      },
    ];
  }, [currencySymbol, overview, t]);

  const dailyColumns = useMemo(
    () => [
      {
        title: t('日期'),
        dataIndex: 'date',
      },
      {
        title: t('单日充值'),
        dataIndex: 'topup_amount',
        render: (value) => `${currencySymbol}${Number(value || 0).toFixed(2)}`,
      },
      {
        title: t('充值笔数'),
        dataIndex: 'topup_count',
        render: (value) => Number(value || 0),
      },
      {
        title: t('单日消耗'),
        dataIndex: 'consume_quota',
        render: (value) => renderQuota(value || 0),
      },
    ],
    [currencySymbol, t],
  );

  const recordColumns = useMemo(
    () => [
      {
        title: 'ID',
        dataIndex: 'id',
        width: 80,
      },
      {
        title: t('用户名'),
        dataIndex: 'username',
        render: (value) => value || '-',
      },
      {
        title: t('订单号'),
        dataIndex: 'trade_no',
        render: (value) => value || '-',
      },
      {
        title: t('充值额度'),
        dataIndex: 'amount',
        render: (value, record) =>
          record?.source === 'redemption'
            ? renderQuota(value || 0)
            : renderQuotaWithAmount(value || 0),
      },
      {
        title: t('支付金额'),
        dataIndex: 'money',
        render: (value) => `${currencySymbol}${Number(value || 0).toFixed(2)}`,
      },
      {
        title: t('支付方式'),
        dataIndex: 'payment_method',
        render: (value) => t(PAYMENT_METHOD_MAP[value] || value || '-'),
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        render: (value) => (
          <Tag color={STATUS_COLOR_MAP[value] || 'grey'} shape='circle'>
            {t(STATUS_LABEL_MAP[value] || value || '-')}
          </Tag>
        ),
      },
      {
        title: t('创建时间'),
        dataIndex: 'create_time',
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('完成时间'),
        dataIndex: 'complete_time',
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
    ],
    [currencySymbol, t],
  );

  const summaryArea = (
    <div className='grid grid-cols-2 md:grid-cols-4 gap-3'>
      {summaryCards.map((item) => (
        <div
          key={item.label}
          className='rounded-2xl border px-4 py-3'
          style={{ borderColor: 'var(--semi-color-border)' }}
        >
          <div className='text-xs text-gray-500 mb-1'>{item.label}</div>
          <div className='text-lg font-semibold break-all'>{item.value}</div>
        </div>
      ))}
    </div>
  );

  const overviewSearchArea = (
    <div className='flex flex-col md:flex-row gap-2 md:items-center md:justify-between'>
      <Space wrap>
        <DatePicker
          type='dateRange'
          value={dateRange}
          onChange={(value) => setDateRange(value || [])}
          placeholder={[t('开始日期'), t('结束日期')]}
        />
        <Button theme='solid' type='primary' onClick={handleSearch}>
          {t('查询')}
        </Button>
        <Button icon={<IconRefresh />} onClick={handleReset}>
          {t('重置')}
        </Button>
      </Space>
      <Text type='secondary'>
        {t('用于统计区间充值、消耗和单日数据')}
      </Text>
    </div>
  );

  const recordsDescriptionArea = (
    <div className='flex items-center text-blue-500'>
      <IconCreditCard className='mr-2' />
      <Text>{t('充值记录')}</Text>
    </div>
  );

  const recordsActionsArea = (
    <div className='flex flex-col md:flex-row gap-2 md:items-center md:justify-between'>
      <Text type='secondary'>{t('管理员可按用户、订单号和状态筛选')}</Text>
      <Space wrap>
        <Input
          prefix={<IconSearch />}
          placeholder={`${t('用户名')}/${t('订单号')}`}
          value={keyword}
          onChange={(value) => setKeyword(value)}
          onEnterPress={handleSearch}
          style={{ width: 240 }}
        />
        <Select
          value={status}
          onChange={(value) => setStatus(value)}
          style={{ width: 140 }}
        >
          <Select.Option value=''>{t('全部状态')}</Select.Option>
          <Select.Option value='success'>{t('成功')}</Select.Option>
          <Select.Option value='pending'>{t('处理中')}</Select.Option>
          <Select.Option value='unpaid'>{t('未支付')}</Select.Option>
          <Select.Option value='expired'>{t('已过期')}</Select.Option>
          <Select.Option value='failed'>{t('支付失败')}</Select.Option>
        </Select>
        <Button theme='solid' type='primary' onClick={handleSearch}>
          {t('筛选')}
        </Button>
      </Space>
    </div>
  );

  const emptyNode = (
    <Empty
      image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
      darkModeImage={
        <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
      }
      description={t('暂无数据')}
      style={{ padding: 30 }}
    />
  );

  return (
    <div className='mt-[60px] px-4 space-y-4'>
      <CardPro
        type='type2'
        statsArea={summaryArea}
        searchArea={overviewSearchArea}
        t={t}
      >
        <Tabs type='card' keepDOM={false}>
          <TabPane
            itemKey='daily'
            tab={
              <span className='flex items-center'>
                <IconCalendar className='mr-2' />
                {t('单日充值 / 消耗')}
              </span>
            }
          >
            <CardTable
              columns={dailyColumns}
              dataSource={overview?.daily_stats || []}
              loading={overviewLoading}
              rowKey='date'
              hidePagination={true}
              scroll={isMobile ? undefined : { x: '100%' }}
              empty={emptyNode}
            />
          </TabPane>
          <TabPane
            itemKey='records'
            tab={
              <span className='flex items-center'>
                <IconCreditCard className='mr-2' />
                {t('充值记录')}
              </span>
            }
          >
            <CardPro
              type='type1'
              descriptionArea={recordsDescriptionArea}
              actionsArea={recordsActionsArea}
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
                columns={recordColumns}
                dataSource={records}
                loading={recordsLoading}
                rowKey={(record) => `${record.source}-${record.id}`}
                hidePagination={true}
                scroll={isMobile ? undefined : { x: '100%' }}
                empty={emptyNode}
              />
            </CardPro>
          </TabPane>
        </Tabs>
      </CardPro>
    </div>
  );
};

export default AdminTopupOverview;
