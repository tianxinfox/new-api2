import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Tag,
  Input,
  TextArea,
  Button,
  Space,
  Typography,
  Empty,
  DatePicker,
  Card,
  Tabs,
  TabPane,
} from '@douyinfe/semi-ui';
import { IconSearch, IconRefresh, IconCoinMoneyStroked } from '@douyinfe/semi-icons';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { useTranslation } from 'react-i18next';
import {
  API,
  renderQuota,
  showError,
  showSuccess,
  timestamp2string,
  getCurrencyConfig,
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

const AgentRebates = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const currencySymbol = getCurrencyConfig().symbol || '$';

  const [records, setRecords] = useState([]);
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [total, setTotal] = useState(0);
  const [keyword, setKeyword] = useState('');
  const [searchKeyword, setSearchKeyword] = useState('');
  const [dateRange, setDateRange] = useState(null);

  const [withdrawStats, setWithdrawStats] = useState(null);
  const [withdrawals, setWithdrawals] = useState([]);
  const [withdrawLoading, setWithdrawLoading] = useState(false);
  const [withdrawPage, setWithdrawPage] = useState(1);
  const [withdrawPageSize, setWithdrawPageSize] = useState(10);
  const [withdrawTotal, setWithdrawTotal] = useState(0);
  const [creatingWithdraw, setCreatingWithdraw] = useState(false);
  const [savingWithdrawAccount, setSavingWithdrawAccount] = useState(false);
  const [activeRecordTab, setActiveRecordTab] = useState('rebates');
  const [withdrawAccount, setWithdrawAccount] = useState({
    payee_account: '',
    payee_name: '',
  });
  const [withdrawForm, setWithdrawForm] = useState({
    amount: '',
    applicant_remark: '',
  });

  const getDefaultRange = () => {
    const now = new Date();
    const start = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0);
    return [start, now];
  };

  const normalizeRangeParams = useCallback((range) => {
    if (!range || range.length !== 2) {
      return null;
    }
    const [start, end] = range;
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
      startTs: Math.floor(normalizedStart.getTime() / 1000),
      endTs: Math.floor(normalizedEnd.getTime() / 1000),
    };
  }, []);

  const loadStats = useCallback(async (range) => {
    try {
      const params = normalizeRangeParams(range || getDefaultRange());
      const res = await API.get(
        `/api/agent/rebates/stats?start_timestamp=${params.startTs}&end_timestamp=${params.endTs}`,
      );
      if (res.data.success) {
        setStats(res.data.data || null);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err.message);
    }
  }, [normalizeRangeParams]);

  const loadRecords = useCallback(async (page, size, search, range) => {
    setLoading(true);
    try {
      const params = [`p=${page}`, `page_size=${size}`];
      if (search) {
        params.push(`keyword=${encodeURIComponent(search)}`);
      }
      const tsParams = normalizeRangeParams(range);
      if (tsParams) {
        params.push(`start_timestamp=${tsParams.startTs}`);
        params.push(`end_timestamp=${tsParams.endTs}`);
      }
      const res = await API.get(`/api/agent/rebates?${params.join('&')}`);
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
  }, [normalizeRangeParams]);

  const loadWithdrawStats = useCallback(async () => {
    try {
      const res = await API.get('/api/agent/withdrawals/stats');
      if (res.data.success) {
        setWithdrawStats(res.data.data || null);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err.message);
    }
  }, []);

  const loadWithdrawAccount = useCallback(async () => {
    try {
      const res = await API.get('/api/user/self');
      if (res.data.success) {
        const data = res.data.data || {};
        setWithdrawAccount({
          payee_account: data.withdraw_payee_account || '',
          payee_name: data.withdraw_payee_name || '',
        });
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err.message);
    }
  }, []);

  const loadWithdrawals = useCallback(async (page, size, range) => {
    setWithdrawLoading(true);
    try {
      const params = [`p=${page}`, `page_size=${size}`];
      const tsParams = normalizeRangeParams(range);
      if (tsParams) {
        params.push(`start_timestamp=${tsParams.startTs}`);
        params.push(`end_timestamp=${tsParams.endTs}`);
      }
      const res = await API.get(`/api/agent/withdrawals?${params.join('&')}`);
      if (res.data.success) {
        const data = res.data.data || {};
        setWithdrawals(data.items || []);
        setWithdrawTotal(data.total || 0);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err.message);
    } finally {
      setWithdrawLoading(false);
    }
  }, [normalizeRangeParams]);

  const reloadAll = useCallback(() => {
    loadStats(dateRange);
    loadRecords(activePage, pageSize, searchKeyword, dateRange);
    loadWithdrawStats();
    loadWithdrawals(withdrawPage, withdrawPageSize, dateRange);
    loadWithdrawAccount();
  }, [
    activePage,
    dateRange,
    loadRecords,
    loadStats,
    loadWithdrawStats,
    loadWithdrawals,
    loadWithdrawAccount,
    pageSize,
    searchKeyword,
    withdrawPage,
    withdrawPageSize,
  ]);

  useEffect(() => {
    loadRecords(activePage, pageSize, searchKeyword, dateRange);
  }, [activePage, pageSize, searchKeyword, dateRange, loadRecords]);

  useEffect(() => {
    loadStats(dateRange);
    loadWithdrawStats();
    loadWithdrawAccount();
  }, [dateRange, loadStats, loadWithdrawStats, loadWithdrawAccount]);

  useEffect(() => {
    loadWithdrawals(withdrawPage, withdrawPageSize, dateRange);
  }, [withdrawPage, withdrawPageSize, dateRange, loadWithdrawals]);

  const handleSearch = () => {
    setSearchKeyword(keyword);
    setActivePage(1);
  };

  const handleReset = () => {
    setKeyword('');
    setSearchKeyword('');
    setActivePage(1);
    setWithdrawPage(1);
    setDateRange(null);
    loadStats(null);
    loadWithdrawStats();
  };

  const handleCreateWithdrawal = async () => {
    setCreatingWithdraw(true);
    try {
      if (!withdrawAvailable) {
        throw new Error(t('支付宝提现配置未完成'));
      }
      const amount = Number(withdrawForm.amount);
      if (!Number.isFinite(amount) || amount <= 0) {
        throw new Error(t('请输入有效提现金额'));
      }
      if (amount < Number(withdrawStats?.min_amount || 0)) {
        throw new Error(
          t('提现金额不能低于 {{amount}}', {
            amount: Number(withdrawStats?.min_amount || 0).toFixed(2),
          }),
        );
      }
      if (!String(withdrawAccount.payee_account || '').trim()) {
        throw new Error(t('请先绑定支付宝账号'));
      }
      if (!String(withdrawAccount.payee_name || '').trim()) {
        throw new Error(t('请先绑定收款人姓名'));
      }
      const res = await API.post('/api/agent/withdrawals', {
        amount,
        applicant_remark: String(withdrawForm.applicant_remark || '').trim(),
      });
      if (!res.data.success) {
        throw new Error(res.data.message);
      }
      showSuccess(t('提现申请已提交'));
      setWithdrawForm({
        amount: '',
        applicant_remark: '',
      });
      setWithdrawPage(1);
      reloadAll();
    } catch (err) {
      showError(err.message);
    } finally {
      setCreatingWithdraw(false);
    }
  };

  const handleSaveWithdrawAccount = async () => {
    setSavingWithdrawAccount(true);
    try {
      const payeeAccount = String(withdrawAccount.payee_account || '').trim();
      const payeeName = String(withdrawAccount.payee_name || '').trim();
      if (!payeeAccount) {
        throw new Error(t('支付宝账号不能为空'));
      }
      if (!payeeName) {
        throw new Error(t('收款人姓名不能为空'));
      }
      const res = await API.put('/api/user/self', {
        withdraw_payee_account: payeeAccount,
        withdraw_payee_name: payeeName,
      });
      if (!res.data.success) {
        throw new Error(res.data.message);
      }
      showSuccess(t('提现账户已保存'));
      setWithdrawAccount({
        payee_account: payeeAccount,
        payee_name: payeeName,
      });
    } catch (err) {
      showError(err.message);
    } finally {
      setSavingWithdrawAccount(false);
    }
  };

  const rebateColumns = useMemo(
    () => [
      {
        title: 'ID',
        dataIndex: 'id',
        width: 80,
      },
      {
        title: t('时间'),
        dataIndex: 'created_at',
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('下游用户'),
        dataIndex: 'username',
        render: (value, record) => value || `#${record.sub_user_id}`,
      },
      {
        title: t('来源'),
        dataIndex: 'source_type',
        render: (value) =>
          value === 'redemption' ? (
            <Tag color='orange' shape='circle'>
              {t('兑换码充值')}
            </Tag>
          ) : (
            <Tag color='green' shape='circle'>
              {t('在线充值')}
            </Tag>
          ),
      },
      {
        title: t('订单号'),
        dataIndex: 'trade_no',
        render: (value) => value || '-',
      },
      {
        title: t('充值金额'),
        dataIndex: 'source_money',
        render: (value) => `${currencySymbol}${Number(value || 0).toFixed(2)}`,
      },
      {
        title: t('返利比例'),
        dataIndex: 'rebate_rate',
        render: (value) => `${(Number(value || 0) / 100).toFixed(2)}%`,
      },
      {
        title: t('返利金额'),
        dataIndex: 'rebate_money',
        render: (value) => `${currencySymbol}${Number(value || 0).toFixed(2)}`,
      },
      {
        title: t('返利额度'),
        dataIndex: 'rebate_quota',
        render: (value) => renderQuota(value || 0),
      },
    ],
    [t, currencySymbol],
  );

  const withdrawalColumns = useMemo(
    () => [
      { title: 'ID', dataIndex: 'id', width: 80 },
      {
        title: t('申请时间'),
        dataIndex: 'created_at',
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('提现金额'),
        dataIndex: 'amount',
        render: (value) => `${currencySymbol}${Number(value || 0).toFixed(2)}`,
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
        render: (value) => value || '-',
      },
      {
        title: t('收款人'),
        dataIndex: 'payee_name',
        render: (value) => value || '-',
      },
      {
        title: t('转账单号'),
        dataIndex: 'alipay_order_id',
        render: (value) => value || '-',
      },
      {
        title: t('审核备注'),
        dataIndex: 'admin_remark',
        render: (value) => value || '-',
      },
      {
        title: t('失败原因'),
        dataIndex: 'failure_reason',
        render: (value) => value || '-',
      },
    ],
    [currencySymbol, t],
  );

  const descriptionArea = (
    <div className='flex items-center text-blue-500'>
      <IconCoinMoneyStroked className='mr-2' />
      <Text>{t('返利明细')}</Text>
    </div>
  );

  const actionsArea = (
    <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
      <Space wrap>
        <DatePicker
          type='dateRange'
          density='compact'
          value={dateRange}
          onChange={setDateRange}
          placeholder={t('选择时间范围')}
          style={{ width: 260 }}
        />
      </Space>
      <Space>
        <Input
          prefix={<IconSearch />}
          placeholder={`${t('下游用户')}/${t('订单号')}`}
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

  const withdrawSummary = withdrawStats?.stats || {};
  const withdrawAvailable = Boolean(withdrawStats?.enabled && withdrawStats?.configured);
  const withdrawAccountBound = Boolean(
    String(withdrawAccount.payee_account || '').trim() && String(withdrawAccount.payee_name || '').trim(),
  );

  return (
    <div className='mt-[60px] px-4 pb-4'>
      <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-4'>
        <Card className='!rounded-xl border-0 shadow-sm'>
          <Text type='tertiary'>{t('区间返利金额')}</Text>
          <div className='text-2xl font-semibold mt-2'>
            {`${currencySymbol}${Number(stats?.range_rebate_money || 0).toFixed(2)}`}
          </div>
        </Card>
        <Card className='!rounded-xl border-0 shadow-sm'>
          <Text type='tertiary'>{t('累计返利金额')}</Text>
          <div className='text-2xl font-semibold mt-2'>
            {`${currencySymbol}${Number(stats?.total_rebate_money || 0).toFixed(2)}`}
          </div>
        </Card>
        <Card className='!rounded-xl border-0 shadow-sm'>
          <Text type='tertiary'>{t('可提现余额')}</Text>
          <div className='text-2xl font-semibold mt-2'>
            {`${currencySymbol}${Number(withdrawSummary.withdrawable_amount || 0).toFixed(2)}`}
          </div>
        </Card>
        <Card className='!rounded-xl border-0 shadow-sm'>
          <Text type='tertiary'>{t('已打款金额')}</Text>
          <div className='text-2xl font-semibold mt-2'>
            {`${currencySymbol}${Number(withdrawSummary.paid_amount || 0).toFixed(2)}`}
          </div>
        </Card>
      </div>

      <Card className='!rounded-2xl border-0 shadow-sm mb-4'>
        <div className='flex items-center justify-between gap-4 flex-wrap mb-4'>
          <div>
            <Text strong>{t('代理提现')}</Text>
            <div className='text-sm text-gray-500 mt-1'>
              {withdrawStats?.enabled
                ? withdrawStats?.configured
                  ? withdrawAccountBound
                    ? t('管理员审核通过后会调用支付宝单笔转账接口打款。')
                    : t('请先绑定提现支付宝账号和实名，再提交提现申请。')
                  : t('管理员尚未完成支付宝提现配置，暂时不能提交申请。')
                : t('管理员暂未启用代理提现。')}
              {withdrawStats?.enabled && (
                <span className='ml-2'>
                  {t('最小提现金额')} {currencySymbol}
                  {Number(withdrawStats?.min_amount || 0).toFixed(2)}
                </span>
              )}
            </div>
          </div>
          <Space wrap>
            <Tag color='blue'>{t('待审核')}: {currencySymbol}{Number(withdrawSummary.pending_amount || 0).toFixed(2)}</Tag>
            <Tag color='cyan'>{t('转账中')}: {currencySymbol}{Number(withdrawSummary.transferring_amount || 0).toFixed(2)}</Tag>
            <Tag color='red'>{t('失败')}: {currencySymbol}{Number(withdrawSummary.failed_amount || 0).toFixed(2)}</Tag>
          </Space>
        </div>

        <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4'>
          <Input
            value={withdrawAccount.payee_account}
            onChange={(value) => setWithdrawAccount((prev) => ({ ...prev, payee_account: value }))}
            placeholder={t('绑定支付宝账号')}
          />
          <Input
            value={withdrawAccount.payee_name}
            onChange={(value) => setWithdrawAccount((prev) => ({ ...prev, payee_name: value }))}
            placeholder={t('绑定收款人真实姓名')}
          />
          <Button theme='solid' loading={savingWithdrawAccount} onClick={handleSaveWithdrawAccount}>
            {t('保存提现账户')}
          </Button>
        </div>
        <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mt-4'>
          <Input
            value={withdrawForm.amount}
            onChange={(value) => setWithdrawForm((prev) => ({ ...prev, amount: value }))}
            placeholder={t('提现金额')}
            disabled={!withdrawAvailable || !withdrawAccountBound}
          />
          <Button
            theme='solid'
            type='primary'
            loading={creatingWithdraw}
            disabled={!withdrawAvailable || !withdrawAccountBound}
            onClick={handleCreateWithdrawal}
          >
            {t('提交提现申请')}
          </Button>
        </div>
        <div className='mt-4'>
          <TextArea
            autosize
            value={withdrawForm.applicant_remark}
            onChange={(value) => setWithdrawForm((prev) => ({ ...prev, applicant_remark: value }))}
            placeholder={t('申请备注（可选）')}
            disabled={!withdrawAvailable || !withdrawAccountBound}
          />
        </div>
      </Card>

      <div className='mt-4'>
        <Tabs activeKey={activeRecordTab} onChange={setActiveRecordTab} type='card'>
          <TabPane tab={t('返利明细')} itemKey='rebates'>
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
                columns={rebateColumns}
                dataSource={records}
                loading={loading}
                rowKey={(item) => `${item.source_type}-${item.id}`}
                scroll={isMobile ? undefined : { x: '100%' }}
                hidePagination={true}
                empty={
                  <Empty
                    image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
                    darkModeImage={<IllustrationNoResultDark style={{ width: 150, height: 150 }} />}
                    description={t('暂无返利记录')}
                    style={{ padding: 30 }}
                  />
                }
                className='overflow-hidden'
                size='middle'
              />
            </CardPro>
          </TabPane>
          <TabPane tab={t('提现记录')} itemKey='withdrawals'>
            <CardPro
              type='type1'
              descriptionArea={<Text>{t('提现记录')}</Text>}
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
                columns={withdrawalColumns}
                dataSource={withdrawals}
                loading={withdrawLoading}
                rowKey='id'
                scroll={isMobile ? undefined : { x: '100%' }}
                hidePagination={true}
                empty={
                  <Empty
                    image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
                    darkModeImage={<IllustrationNoResultDark style={{ width: 150, height: 150 }} />}
                    description={t('暂无提现记录')}
                    style={{ padding: 30 }}
                  />
                }
              />
            </CardPro>
          </TabPane>
        </Tabs>
      </div>
    </div>
  );
};

export default AgentRebates;
