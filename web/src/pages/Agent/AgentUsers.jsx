import React, { useState, useEffect, useCallback } from 'react';
import {
  Card,
  Table,
  Tag,
  Input,
  Button,
  Space,
  Typography,
  Banner,
} from '@douyinfe/semi-ui';
import { IconSearch } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import {
  API,
  renderQuota,
  renderNumber,
  showError,
  timestamp2string,
} from '../../helpers';

const { Text } = Typography;

const renderStatus = (record, t) => {
  const tagColor = record.is_active ? 'green' : 'red';
  const tagText = record.is_active ? t('已激活') : t('未激活');
  return (
    <Tag color={tagColor} shape='circle' size='small'>
      {tagText}
    </Tag>
  );
};

const renderStatistics = (record, t) => {
  const stats = record.statistics || {};
  return (
    <Space spacing={4} wrap>
      <Tag color='white' shape='circle' className='!text-xs'>
        {t('剩余额度')}: {renderQuota(stats.quota || 0)}
      </Tag>
      <Tag color='white' shape='circle' className='!text-xs'>
        {t('已用额度')}: {renderQuota(stats.used_quota || 0)}
      </Tag>
      <Tag color='white' shape='circle' className='!text-xs'>
        {t('调用次数')}: {renderNumber(stats.request_count || 0)}
      </Tag>
    </Space>
  );
};

const renderInviteInfo = (record, t) => {
  const invite = record.invite_info || {};
  return (
    <Space spacing={1}>
      <Tag color='white' shape='circle' className='!text-xs'>
        {t('邀请')}: {renderNumber(invite.aff_count || 0)}
      </Tag>
      <Tag color='white' shape='circle' className='!text-xs'>
        {t('收益')}: {renderQuota(invite.aff_history_quota || 0)}
      </Tag>
    </Space>
  );
};

const AgentUsers = () => {
  const { t } = useTranslation();
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [userCount, setUserCount] = useState(0);
  const [keyword, setKeyword] = useState('');

  const loadUsers = useCallback(
    async (page, size, search) => {
      setLoading(true);
      try {
        let url;
        if (search) {
          url = `/api/agent/users/search?keyword=${encodeURIComponent(search)}&p=${page}&page_size=${size}`;
        } else {
          url = `/api/agent/users?p=${page}&page_size=${size}`;
        }
        const res = await API.get(url);
        if (res.data.success) {
          const data = res.data.data;
          setUsers(data.items || []);
          setUserCount(data.total || 0);
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
    loadUsers(activePage, pageSize, keyword);
  }, [activePage, pageSize]);

  const handleSearch = () => {
    setActivePage(1);
    loadUsers(1, pageSize, keyword);
  };

  const handleReset = () => {
    setKeyword('');
    setActivePage(1);
    loadUsers(1, pageSize, '');
  };

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      width: 80,
    },
    {
      title: t('昵称'),
      dataIndex: 'nickname',
      width: 160,
      render: (text) => text || '-',
    },
    {
      title: t('统计信息'),
      key: 'statistics',
      width: 320,
      render: (text, record) => renderStatistics(record, t),
    },
    {
      title: t('邀请信息'),
      key: 'invite',
      width: 220,
      render: (text, record) => renderInviteInfo(record, t),
    },
    {
      title: t('注册时间'),
      dataIndex: 'registered_at',
      width: 180,
      render: (value) => (value ? timestamp2string(value) : '-'),
    },
    {
      title: t('激活状态'),
      key: 'status',
      width: 100,
      render: (text, record) => renderStatus(record, t),
    },
  ];

  return (
    <div className='mt-[60px] px-4 max-w-[1400px] mx-auto'>
      <div className='mb-6'>
        <Text className='text-2xl font-bold'>{t('邀请用户')}</Text>
      </div>

      <Banner
        type='info'
        description={t('邀请用户页面，可以查看通过您的邀请链接注册的所有用户。')}
        className='mb-4 !rounded-xl'
      />

      <Card className='!rounded-2xl border-0 shadow-sm mb-4'>
        <div className='flex items-center gap-3 flex-wrap'>
          <Input
            prefix={<IconSearch />}
            placeholder={t('用户ID/用户名/显示名')}
            value={keyword}
            onChange={(val) => setKeyword(val)}
            onEnterPress={handleSearch}
            style={{ width: 280 }}
          />
          <Button theme='solid' type='primary' onClick={handleSearch}>
            {t('查询')}
          </Button>
          <Button onClick={handleReset}>{t('重置')}</Button>
        </div>
      </Card>

      <Card className='!rounded-2xl border-0 shadow-sm'>
        <Table
          columns={columns}
          dataSource={users}
          loading={loading}
          rowKey='id'
          pagination={{
            currentPage: activePage,
            pageSize: pageSize,
            total: userCount,
            onPageChange: (page) => setActivePage(page),
            onPageSizeChange: (size) => {
              setPageSize(size);
              setActivePage(1);
            },
            showSizeChanger: true,
            pageSizeOpts: [10, 20, 50],
            formatPageText: (pageInfo) =>
              t('第 {{start}}-{{end}} 条，共 {{total}} 条', {
                start: pageInfo.currentStart,
                end: pageInfo.currentEnd,
                total: userCount,
              }),
          }}
          scroll={{ x: 'max-content' }}
          empty={
            <div className='text-center text-gray-400 py-8'>
              {t('暂无数据')}
            </div>
          }
        />
      </Card>
    </div>
  );
};

export default AgentUsers;
