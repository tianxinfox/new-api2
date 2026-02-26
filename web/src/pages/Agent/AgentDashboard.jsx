import React, { useState, useEffect, useCallback } from 'react';
import {
  Card,
  DatePicker,
  Button,
  Skeleton,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, renderQuota, showError, getCurrencyConfig } from '../../helpers';

const { Text } = Typography;

const AgentDashboard = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [stats, setStats] = useState(null);
  const [dateRange, setDateRange] = useState(null);

  const getDefaultRange = () => {
    const now = new Date();
    const start = new Date(
      now.getFullYear(),
      now.getMonth(),
      now.getDate() - 1,
      0,
      0,
      0,
    );
    return [start, now];
  };

  const loadData = useCallback(
    async (range) => {
      setLoading(true);
      try {
        const [start, end] = range || getDefaultRange();
        const startTs = Math.floor(start.getTime() / 1000);
        const endTs = Math.floor(end.getTime() / 1000);
        const res = await API.get(
          `/api/agent/dashboard?start_timestamp=${startTs}&end_timestamp=${endTs}`,
        );
        if (res.data.success) {
          setStats(res.data.data);
        } else {
          showError(res.data.message);
        }
      } catch (err) {
        showError(err.message);
      } finally {
        setLoading(false);
      }
    },
    [],
  );

  useEffect(() => {
    loadData(dateRange);
  }, []);

  const handleQuery = () => {
    loadData(dateRange);
  };
  const currencySymbol = getCurrencyConfig().symbol || '$';

  const statCards = [
    {
      label: t('今日充值'),
      value: stats ? `${currencySymbol}${Number(stats.today_topup || 0).toFixed(2)}` : '-',
      color: 'bg-emerald-50',
      textColor: 'text-emerald-600',
    },
    {
      label: t('今日消费'),
      value: stats ? renderQuota(stats.today_consumption) : '-',
      color: 'bg-orange-50',
      textColor: 'text-orange-600',
    },
    {
      label: t('今日注册'),
      value: stats ? stats.today_registrations : '-',
      color: 'bg-blue-50',
      textColor: 'text-blue-600',
    },
    {
      label: t('今日代理注册'),
      value: stats ? stats.today_agent_registrations : '-',
      color: 'bg-purple-50',
      textColor: 'text-purple-600',
    },
  ];

  const renderRankingTable = (
    title,
    keyLabel,
    data,
    valueLabel,
    isQuota = false,
  ) => (
    <Card className='!rounded-2xl border-0 shadow-sm' bodyStyle={{ padding: '20px 24px' }}>
      <div className='flex items-center justify-between mb-5'>
        <Text strong className='text-base'>
          {title}
        </Text>
      </div>
      <div className='space-y-2'>
        <div className='flex items-center justify-between text-xs text-gray-400 pb-2 border-b'>
          <span>{keyLabel}</span>
          <span>{valueLabel}</span>
        </div>
        {data && data.length > 0 ? (
          data.map((item, idx) => (
            <div
              key={idx}
              className='flex items-center justify-between py-2'
            >
              <span className='text-sm text-gray-700 truncate max-w-[60%]'>
                {item.name || '-'}
              </span>
              <span className='text-sm font-medium'>
                {isQuota ? renderQuota(item.value) : item.value}
              </span>
            </div>
          ))
        ) : (
          <div className='text-center text-gray-400 py-6 text-sm'>
            {t('暂无数据')}
          </div>
        )}
      </div>
    </Card>
  );

  return (
    <div className='mt-[60px] px-6 pb-8 max-w-[1400px] mx-auto'>
      <div className='mb-8'>
        <Text className='text-2xl font-bold'>{t('代理面板')}</Text>
      </div>

      <div className='mb-8'>
        <Card className='!rounded-2xl border-0 shadow-sm'>
          <div className='flex items-center gap-4 flex-wrap'>
            <span className='text-sm text-gray-500'>{t('选择日期')}:</span>
            <DatePicker
              type='dateRange'
              density='compact'
              placeholder={t('默认今天')}
              onChange={(dates) => setDateRange(dates)}
              style={{ width: 260 }}
            />
            <Button
              theme='solid'
              type='primary'
              onClick={handleQuery}
              loading={loading}
            >
              {t('查询')}
            </Button>
            <span className='text-xs text-gray-400'>
              {t('默认查询今天及昨天的数据')}
            </span>
          </div>
        </Card>
      </div>

      <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8'>
        {statCards.map((card, idx) => (
          <Card
            key={idx}
            className={`!rounded-2xl border-0 shadow-sm ${card.color}`}
            bodyStyle={{ padding: '20px 24px' }}
          >
            <div className='text-sm text-gray-500 mb-2'>{card.label}</div>
            <Skeleton loading={loading} active placeholder={<Skeleton.Paragraph rows={1} style={{ width: 80, height: 32 }} />}>
              <div className={`text-2xl font-bold ${card.textColor}`}>
                {card.value}
              </div>
            </Skeleton>
          </Card>
        ))}
      </div>

      <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6'>
        {renderRankingTable(
          t('模型排行'),
          t('模型'),
          stats?.model_ranking,
          t('使用量'),
          true,
        )}
        {renderRankingTable(
          t('渠道排行'),
          t('渠道'),
          stats?.channel_ranking,
          t('使用量'),
          true,
        )}
        {renderRankingTable(
          t('用户排行'),
          t('用户'),
          stats?.user_ranking,
          t('使用量'),
          true,
        )}
        {renderRankingTable(
          t('错误模型排行'),
          t('模型'),
          stats?.error_ranking,
          t('错误次数'),
          false,
        )}
      </div>
    </div>
  );
};

export default AgentDashboard;
