import React, { useState, useEffect, useCallback, useMemo } from 'react';
import {
  Tag,
  Input,
  Button,
  Space,
  Typography,
  Empty,
  DatePicker,
  Card,
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
  timestamp2string,
  getCurrencyConfig,
} from '../../helpers';
import CardPro from '../../components/common/ui/CardPro';
import CardTable from '../../components/common/ui/CardTable';
import { createCardProPagination } from '../../helpers/utils';
import { useIsMobile } from '../../hooks/common/useIsMobile';

const { Text } = Typography;

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

  const getDefaultRange = () => {
    const now = new Date();
    const start = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0);
    return [start, now];
  };

  const loadStats = useCallback(async (range) => {
    try {
      const [start, end] = range || getDefaultRange();
      const startTs = Math.floor(start.getTime() / 1000);
      const endTs = Math.floor(end.getTime() / 1000);
      const res = await API.get(`/api/agent/rebates/stats?start_timestamp=${startTs}&end_timestamp=${endTs}`);
      if (res.data.success) {
        setStats(res.data.data || null);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err.message);
    }
  }, []);

  const loadRecords = useCallback(async (page, size, search) => {
    setLoading(true);
    try {
      const params = [`p=${page}`, `page_size=${size}`];
      if (search) {
        params.push(`keyword=${encodeURIComponent(search)}`);
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
  }, []);

  useEffect(() => {
    loadRecords(activePage, pageSize, searchKeyword);
  }, [activePage, pageSize, searchKeyword, loadRecords]);

  useEffect(() => {
    loadStats(dateRange);
  }, [dateRange, loadStats]);

  const handleSearch = () => {
    setSearchKeyword(keyword);
    setActivePage(1);
  };

  const handleReset = () => {
    setKeyword('');
    setSearchKeyword('');
    setActivePage(1);
    setDateRange(null);
    loadStats(null);
  };

  const columns = useMemo(
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
          <Text type='tertiary'>{t('区间返利额度')}</Text>
          <div className='text-2xl font-semibold mt-2'>
            {renderQuota(stats?.range_rebate_quota || 0)}
          </div>
        </Card>
        <Card className='!rounded-xl border-0 shadow-sm'>
          <Text type='tertiary'>{t('累计返利金额')}</Text>
          <div className='text-2xl font-semibold mt-2'>
            {`${currencySymbol}${Number(stats?.total_rebate_money || 0).toFixed(2)}`}
          </div>
        </Card>
        <Card className='!rounded-xl border-0 shadow-sm'>
          <Text type='tertiary'>{t('累计返利额度')}</Text>
          <div className='text-2xl font-semibold mt-2'>
            {renderQuota(stats?.total_rebate_quota || 0)}
          </div>
        </Card>
      </div>

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
    </div>
  );
};

export default AgentRebates;
