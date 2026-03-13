import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Button, Card, DatePicker, Select, Space, Typography, Tag, Tabs, TabPane } from '@douyinfe/semi-ui';
import { IconRefresh } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import {
  API,
  getCurrencyConfig,
  renderQuota,
  showError,
  timestamp2string,
} from '../../helpers';
import CardPro from '../../components/common/ui/CardPro';
import CardTable from '../../components/common/ui/CardTable';
import { createCardProPagination } from '../../helpers/utils';
import { useIsMobile } from '../../hooks/common/useIsMobile';

const { Text } = Typography;
const withdrawalStatusColorMap = {
  pending: 'orange',
  rejected: 'red',
  transferring: 'blue',
  paid: 'green',
  failed: 'grey',
};

const withdrawalStatusLabelMap = {
  pending: '待审核',
  rejected: '已拒绝',
  transferring: '转账中',
  paid: '已打款',
  failed: '打款失败',
};

const sortOptions = [
  { label: 'ID', value: 'agent_id' },
  { label: '下级累计充值', value: 'total_topup_amount' },
  { label: '下级累计消费', value: 'total_consumption_quota' },
  { label: '下级累计调用', value: 'total_request_count' },
  { label: '累计返佣', value: 'total_rebate_amount' },
  { label: '净贡献', value: 'net_contribution_amount' },
  { label: '最近活跃', value: 'last_active_at' },
];

const rankMetricOptions = [
  { label: '充值榜', value: 'topup' },
  { label: '消费榜', value: 'consumption' },
  { label: '调用榜', value: 'requests' },
  { label: '返佣榜', value: 'rebate' },
  { label: '净贡献榜', value: 'net' },
];

const getDefaultRange = () => {
  const now = new Date();
  const start = new Date(now.getFullYear(), now.getMonth(), 1, 0, 0, 0);
  return [start, now];
};

const AgentOverview = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const currencySymbol = getCurrencyConfig().symbol || '$';

  const [loadingSummary, setLoadingSummary] = useState(false);
  const [loadingList, setLoadingList] = useState(false);
  const [loadingRank, setLoadingRank] = useState(false);
  const [summary, setSummary] = useState(null);
  const [items, setItems] = useState([]);
  const [rankItems, setRankItems] = useState([]);
  const [withdrawLoading, setWithdrawLoading] = useState(false);
  const [withdrawItems, setWithdrawItems] = useState([]);
  const [withdrawPage, setWithdrawPage] = useState(1);
  const [withdrawPageSize, setWithdrawPageSize] = useState(10);
  const [withdrawTotal, setWithdrawTotal] = useState(0);
  const [withdrawStatus, setWithdrawStatus] = useState('');
  const [activeTab, setActiveTab] = useState('overview');

  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [total, setTotal] = useState(0);

  const [dateRange, setDateRange] = useState(getDefaultRange());
  const [sortBy, setSortBy] = useState('total_topup_amount');
  const [sortOrder, setSortOrder] = useState('desc');
  const [appliedDateRange, setAppliedDateRange] = useState(getDefaultRange());
  const [appliedSortBy, setAppliedSortBy] = useState('total_topup_amount');
  const [appliedSortOrder, setAppliedSortOrder] = useState('desc');
  const [rankMetric, setRankMetric] = useState('topup');
  const [refreshKey, setRefreshKey] = useState(0);

  const loading = loadingSummary || loadingList || loadingRank;

  const buildTsQuery = useCallback((range) => {
    const [start, end] = range || [];
    if (!start || !end) {
      return '';
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
    const startTs = Math.floor(normalizedStart.getTime() / 1000);
    const endTs = Math.floor(normalizedEnd.getTime() / 1000);
    return `start_timestamp=${startTs}&end_timestamp=${endTs}`;
  }, []);

  const loadSummary = useCallback(async () => {
    setLoadingSummary(true);
    try {
      const qs = buildTsQuery(appliedDateRange);
      const res = await API.get(`/api/agent/admin/summary${qs ? `?${qs}` : ''}`);
      if (!res.data.success) {
        throw new Error(res.data.message);
      }
      setSummary(res.data.data);
    } catch (error) {
      showError(error.message);
    } finally {
      setLoadingSummary(false);
    }
  }, [appliedDateRange, buildTsQuery]);

  const loadList = useCallback(async (page, size) => {
    setLoadingList(true);
    try {
      const params = [
        `p=${page}`,
        `page_size=${size}`,
        `sort_by=${encodeURIComponent(appliedSortBy)}`,
        `sort_order=${encodeURIComponent(appliedSortOrder)}`,
      ];
      const tsQuery = buildTsQuery(appliedDateRange);
      if (tsQuery) {
        params.push(tsQuery);
      }
      const res = await API.get(`/api/agent/admin/list?${params.join('&')}`);
      if (!res.data.success) {
        throw new Error(res.data.message);
      }
      const data = res.data.data || {};
      setItems(data.items || []);
      setTotal(data.total || 0);
    } catch (error) {
      showError(error.message);
    } finally {
      setLoadingList(false);
    }
  }, [appliedDateRange, appliedSortBy, appliedSortOrder, buildTsQuery]);

  const loadRank = useCallback(async () => {
    setLoadingRank(true);
    try {
      const params = [`metric=${encodeURIComponent(rankMetric)}`, 'limit=10'];
      const tsQuery = buildTsQuery(appliedDateRange);
      if (tsQuery) {
        params.push(tsQuery);
      }
      const res = await API.get(`/api/agent/admin/rank?${params.join('&')}`);
      if (!res.data.success) {
        throw new Error(res.data.message);
      }
      setRankItems(res.data.data || []);
    } catch (error) {
      showError(error.message);
    } finally {
      setLoadingRank(false);
    }
  }, [appliedDateRange, buildTsQuery, rankMetric]);

  const loadWithdrawals = useCallback(async (page, size) => {
    setWithdrawLoading(true);
    try {
      const params = [`p=${page}`, `page_size=${size}`];
      const tsQuery = buildTsQuery(appliedDateRange);
      if (tsQuery) {
        params.push(tsQuery);
      }
      if (withdrawStatus) {
        params.push(`status=${encodeURIComponent(withdrawStatus)}`);
      }
      const res = await API.get(`/api/agent/admin/withdrawals?${params.join('&')}`);
      if (!res.data.success) {
        throw new Error(res.data.message);
      }
      const data = res.data.data || {};
      setWithdrawItems(data.items || []);
      setWithdrawTotal(data.total || 0);
    } catch (error) {
      showError(error.message);
    } finally {
      setWithdrawLoading(false);
    }
  }, [appliedDateRange, buildTsQuery, withdrawStatus]);

  useEffect(() => {
    loadSummary();
  }, [loadSummary, refreshKey]);

  useEffect(() => {
    loadList(activePage, pageSize);
  }, [activePage, pageSize, loadList, refreshKey]);

  useEffect(() => {
    loadRank();
  }, [loadRank, refreshKey]);

  useEffect(() => {
    loadWithdrawals(withdrawPage, withdrawPageSize);
  }, [loadWithdrawals, refreshKey, withdrawPage, withdrawPageSize]);

  const handleQuery = () => {
    setAppliedDateRange(dateRange);
    setAppliedSortBy(sortBy);
    setAppliedSortOrder(sortOrder);
    setActivePage(1);
    setWithdrawPage(1);
    setRefreshKey((prev) => prev + 1);
  };

  const handleReviewWithdrawal = useCallback(async (record, approve) => {
    const adminRemark = window.prompt(
      approve ? t('请输入审核备注（可选）') : t('请输入拒绝原因/备注（可选）'),
      record.admin_remark || '',
    );
    if (adminRemark === null) {
      return;
    }
    try {
      const res = await API.post(`/api/agent/admin/withdrawals/${record.id}/review`, {
        approve,
        admin_remark: adminRemark,
      });
      if (!res.data.success) {
        throw new Error(res.data.message);
      }
      setRefreshKey((prev) => prev + 1);
    } catch (error) {
      showError(error.message);
    }
  }, [t]);

  const handleSyncWithdrawal = useCallback(async (record) => {
    try {
      const res = await API.post(`/api/agent/admin/withdrawals/${record.id}/sync`, {});
      if (!res.data.success) {
        throw new Error(res.data.message);
      }
      setRefreshKey((prev) => prev + 1);
    } catch (error) {
      showError(error.message);
    }
  }, []);

  const handleFailWithdrawal = useCallback(async (record) => {
    const reason = window.prompt(t('请输入标记失败原因（可选）'), record.failure_reason || '');
    if (reason === null) {
      return;
    }
    try {
      const res = await API.post(`/api/agent/admin/withdrawals/${record.id}/fail`, {
        reason,
      });
      if (!res.data.success) {
        throw new Error(res.data.message);
      }
      setRefreshKey((prev) => prev + 1);
    } catch (error) {
      showError(error.message);
    }
  }, [t]);

  const summaryCards = [
    {
      label: t('总代理数'),
      value: summary ? summary.total_agents : '-',
    },
    {
      label: t('总下级用户'),
      value: summary ? summary.total_sub_users : '-',
    },
    {
      label: t('累计充值'),
      value: summary
        ? `${currencySymbol}${Number(summary.total_topup_amount || 0).toFixed(2)}`
        : '-',
    },
    {
      label: t('累计消费'),
      value: summary ? renderQuota(summary.total_consumption_quota || 0) : '-',
    },
    {
      label: t('累计调用'),
      value: summary ? summary.total_request_count : '-',
    },
    {
      label: t('累计返佣'),
      value: summary
        ? `${currencySymbol}${Number(summary.total_rebate_amount || 0).toFixed(2)}`
        : '-',
    },
    {
      label: t('净贡献'),
      value: summary
        ? `${currencySymbol}${Number(summary.net_contribution_amount || 0).toFixed(2)}`
        : '-',
    },
  ];

  const tableColumns = useMemo(() => [
    {
      title: 'ID',
      dataIndex: 'agent_id',
      width: 80,
    },
    {
      title: t('代理'),
      dataIndex: 'agent_name',
      render: (v) => v || '-',
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (v) => (v === 1 ? t('启用') : t('禁用')),
    },
    {
      title: t('下级用户数'),
      dataIndex: 'sub_user_count',
    },
    {
      title: t('下级累计充值'),
      dataIndex: 'total_topup_amount',
      render: (v) => `${currencySymbol}${Number(v || 0).toFixed(2)}`,
    },
    {
      title: t('下级累计消费'),
      dataIndex: 'total_consumption_quota',
      render: (v) => renderQuota(v || 0),
    },
    {
      title: t('下级累计调用'),
      dataIndex: 'total_request_count',
    },
    {
      title: t('累计返佣'),
      dataIndex: 'total_rebate_amount',
      render: (v) => `${currencySymbol}${Number(v || 0).toFixed(2)}`,
    },
    {
      title: t('净贡献'),
      dataIndex: 'net_contribution_amount',
      render: (v) => `${currencySymbol}${Number(v || 0).toFixed(2)}`,
    },
    {
      title: t('最近活跃'),
      dataIndex: 'last_active_at',
      render: (v) => (v > 0 ? timestamp2string(v) : '-'),
    },
  ], [currencySymbol, t]);

  const rankTitleMap = {
    topup: t('充值榜'),
    consumption: t('消费榜'),
    requests: t('调用榜'),
    rebate: t('返佣榜'),
    net: t('净贡献榜'),
  };

  const withdrawColumns = useMemo(() => [
    {
      title: 'ID',
      dataIndex: 'id',
      width: 80,
    },
    {
      title: t('代理'),
      dataIndex: 'agent_name',
      render: (v) => v || '-',
    },
    {
      title: t('申请时间'),
      dataIndex: 'created_at',
      render: (v) => (v > 0 ? timestamp2string(v) : '-'),
    },
    {
      title: t('提现金额'),
      dataIndex: 'amount',
      render: (v) => `${currencySymbol}${Number(v || 0).toFixed(2)}`,
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (value) => (
        <Tag color={withdrawalStatusColorMap[value] || 'grey'} shape='circle'>
          {t(withdrawalStatusLabelMap[value] || value || '-')}
        </Tag>
      ),
    },
    {
      title: t('支付宝账号'),
      dataIndex: 'payee_account',
      render: (v) => v || '-',
    },
    {
      title: t('收款人'),
      dataIndex: 'payee_name',
      render: (v) => v || '-',
    },
    {
      title: t('转账单号'),
      dataIndex: 'alipay_order_id',
      render: (v) => v || '-',
    },
    {
      title: t('备注'),
      dataIndex: 'admin_remark',
      render: (v, record) => v || record.applicant_remark || '-',
    },
    {
      title: t('失败原因'),
      dataIndex: 'failure_reason',
      render: (v) => v || '-',
    },
    {
      title: t('操作'),
      dataIndex: 'operate',
      render: (_, record) => (
        <Space wrap>
          {record.status === 'pending' && (
            <>
              <Button size='small' type='primary' theme='solid' onClick={() => handleReviewWithdrawal(record, true)}>
                {t('审核通过')}
              </Button>
              <Button size='small' type='danger' onClick={() => handleReviewWithdrawal(record, false)}>
                {t('拒绝')}
              </Button>
            </>
          )}
          {record.status === 'failed' && (
            <Button size='small' type='primary' theme='solid' onClick={() => handleReviewWithdrawal(record, true)}>
              {t('重新打款')}
            </Button>
          )}
          {record.status === 'transferring' && (
            <>
              <Button size='small' onClick={() => handleSyncWithdrawal(record)}>
                {t('同步状态')}
              </Button>
              <Button size='small' type='danger' onClick={() => handleFailWithdrawal(record)}>
                {t('标记失败')}
              </Button>
            </>
          )}
        </Space>
      ),
    },
  ], [currencySymbol, handleFailWithdrawal, handleReviewWithdrawal, handleSyncWithdrawal, t]);

  const descriptionArea = (
    <div className='flex items-center'>
      <Text strong>{t('代理总览')}</Text>
    </div>
  );

  const actionsArea = (
    <Space wrap>
      <DatePicker
        type='dateRange'
        density='compact'
        value={dateRange}
        onChange={(dates) => setDateRange(dates)}
        style={{ width: 260 }}
      />
      <Select
        value={sortBy}
        onChange={setSortBy}
        optionList={sortOptions.map((opt) => ({
          label: t(opt.label),
          value: opt.value,
        }))}
        style={{ width: 150 }}
      />
      <Select
        value={sortOrder}
        onChange={setSortOrder}
        optionList={[
          { label: t('降序'), value: 'desc' },
          { label: t('升序'), value: 'asc' },
        ]}
        style={{ width: 100 }}
      />
      <Button theme='solid' type='primary' loading={loading} onClick={handleQuery}>
        {t('查询')}
      </Button>
      <Button icon={<IconRefresh />} onClick={() => setRefreshKey((prev) => prev + 1)}>
        {t('刷新')}
      </Button>
    </Space>
  );

  return (
    <div className='mt-[60px] px-4'>
      <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6'>
        {summaryCards.map((card) => (
          <Card key={card.label} className='!rounded-2xl border-0 shadow-sm'>
            <div className='text-sm text-gray-500 mb-1'>{card.label}</div>
            <div className='text-2xl font-bold'>{card.value}</div>
          </Card>
        ))}
      </div>

      <div className='mt-6'>
        <Tabs activeKey={activeTab} onChange={setActiveTab} type='card'>
          <TabPane tab={t('代理总览')} itemKey='overview'>
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
                columns={tableColumns}
                dataSource={items}
                loading={loadingList}
                rowKey='agent_id'
                hidePagination={true}
                scroll={isMobile ? undefined : { x: '100%' }}
              />
            </CardPro>
          </TabPane>
          <TabPane tab={t('充值榜')} itemKey='ranking'>
            <Card className='!rounded-2xl border-0 shadow-sm'>
              <div className='flex items-center justify-between mb-4'>
                <Text strong>{rankTitleMap[rankMetric]}</Text>
                <Select
                  value={rankMetric}
                  onChange={setRankMetric}
                  optionList={rankMetricOptions.map((opt) => ({
                    label: t(opt.label),
                    value: opt.value,
                  }))}
                  style={{ width: 140 }}
                />
              </div>
              <div className='space-y-2'>
                {rankItems.map((item, idx) => (
                  <div key={item.agent_id} className='flex items-center justify-between border-b border-gray-100 py-2'>
                    <div className='text-sm text-gray-600'>
                      {idx + 1}. {item.agent_name}
                    </div>
                    <div className='text-sm font-medium'>
                      {rankMetric === 'consumption'
                        ? renderQuota(item.total_consumption_quota || 0)
                        : rankMetric === 'requests'
                          ? item.total_request_count || 0
                          : rankMetric === 'rebate'
                            ? `${currencySymbol}${Number(item.total_rebate_amount || 0).toFixed(2)}`
                            : rankMetric === 'net'
                              ? `${currencySymbol}${Number(item.net_contribution_amount || 0).toFixed(2)}`
                              : `${currencySymbol}${Number(item.total_topup_amount || 0).toFixed(2)}`}
                    </div>
                  </div>
                ))}
                {rankItems.length === 0 && (
                  <div className='text-sm text-gray-400 text-center py-8'>{t('暂无数据')}</div>
                )}
              </div>
            </Card>
          </TabPane>
          <TabPane tab={t('代理提现审核')} itemKey='withdrawals'>
            <CardPro
              type='type1'
              descriptionArea={<Text strong>{t('代理提现审核')}</Text>}
              actionsArea={
                <Space wrap>
                  <Select
                    value={withdrawStatus}
                    onChange={setWithdrawStatus}
                    optionList={[
                      { label: t('全部状态'), value: '' },
                      { label: t('待审核'), value: 'pending' },
                      { label: t('转账中'), value: 'transferring' },
                      { label: t('已打款'), value: 'paid' },
                      { label: t('打款失败'), value: 'failed' },
                      { label: t('已拒绝'), value: 'rejected' },
                    ]}
                    style={{ width: 140 }}
                  />
                  <Button icon={<IconRefresh />} onClick={() => setRefreshKey((prev) => prev + 1)}>
                    {t('刷新')}
                  </Button>
                </Space>
              }
              paginationArea={createCardProPagination({
                currentPage: withdrawPage,
                pageSize: withdrawPageSize,
                total: withdrawTotal,
                onPageChange: setWithdrawPage,
                onPageSizeChange: (size) => {
                  setWithdrawPageSize(size);
                  setWithdrawPage(1);
                },
                isMobile,
                t,
              })}
              t={t}
            >
              <CardTable
                columns={withdrawColumns}
                dataSource={withdrawItems}
                loading={withdrawLoading}
                rowKey='id'
                hidePagination={true}
                scroll={isMobile ? undefined : { x: '100%' }}
              />
            </CardPro>
          </TabPane>
        </Tabs>
      </div>
    </div>
  );
};

export default AgentOverview;
