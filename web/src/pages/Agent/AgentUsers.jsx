import React, { useState, useEffect, useCallback, useMemo } from 'react';
import {
  Tag,
  Input,
  Button,
  Space,
  Typography,
  Tooltip,
  Progress,
  Popover,
  Empty,
} from '@douyinfe/semi-ui';
import {
  IconSearch,
  IconRefresh,
  IconUserGroup,
} from '@douyinfe/semi-icons';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { useTranslation } from 'react-i18next';
import {
  API,
  renderQuota,
  renderNumber,
  renderGroup,
  showError,
  timestamp2string,
} from '../../helpers';
import CardPro from '../../components/common/ui/CardPro';
import CardTable from '../../components/common/ui/CardTable';
import { createCardProPagination } from '../../helpers/utils';
import { useIsMobile } from '../../hooks/common/useIsMobile';

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

const renderQuotaUsage = (record, t) => {
  const stats = record.statistics || {};
  const used = parseInt(stats.used_quota) || 0;
  const remain = parseInt(stats.quota) || 0;
  const total = used + remain;
  const percent = total > 0 ? (remain / total) * 100 : 0;

  const popoverContent = (
    <div className='text-xs p-2'>
      <div>
        {t('已用额度')}: {renderQuota(used)}
      </div>
      <div>
        {t('剩余额度')}: {renderQuota(remain)} ({percent.toFixed(0)}%)
      </div>
      <div>
        {t('总额度')}: {renderQuota(total)}
      </div>
    </div>
  );

  return (
    <Popover content={popoverContent} position='top'>
      <Tag color='white' shape='circle'>
        <div className='flex flex-col items-end'>
          <span className='text-xs leading-none'>{`${renderQuota(remain)} / ${renderQuota(total)}`}</span>
          <Progress
            percent={percent}
            aria-label='quota usage'
            format={() => `${percent.toFixed(0)}%`}
            style={{ width: '100%', marginTop: '1px', marginBottom: 0 }}
          />
        </div>
      </Tag>
    </Popover>
  );
};

const renderStatistics = (record, t) => {
  const stats = record.statistics || {};
  const tooltipContent = (
    <div className='text-xs'>
      <div>
        {t('调用次数')}: {renderNumber(stats.request_count || 0)}
      </div>
    </div>
  );

  const tagColor = record.is_active ? 'green' : 'red';
  const tagText = record.is_active ? t('已激活') : t('未激活');

  return (
    <Tooltip content={tooltipContent} position='top'>
      <Tag color={tagColor} shape='circle' size='small'>
        {tagText}
      </Tag>
    </Tooltip>
  );
};

const renderRole = (role, t) => {
  switch (role) {
    case 1:
      return (
        <Tag color='blue' shape='circle'>
          {t('普通用户')}
        </Tag>
      );
    case 5:
      return (
        <Tag color='green' shape='circle'>
          {t('代理')}
        </Tag>
      );
    case 10:
      return (
        <Tag color='yellow' shape='circle'>
          {t('管理员')}
        </Tag>
      );
    case 100:
      return (
        <Tag color='orange' shape='circle'>
          {t('超级管理员')}
        </Tag>
      );
    default:
      return (
        <Tag color='red' shape='circle'>
          {t('未知身份')}
        </Tag>
      );
  }
};

const renderInviteInfo = (record, t) => {
  const invite = record.invite_info || {};
  return (
    <Space spacing={1} wrap>
      <Tag color='white' shape='circle' className='!text-xs'>
        {t('邀请')}: {renderNumber(invite.aff_count || 0)}
      </Tag>
      <Tag color='white' shape='circle' className='!text-xs'>
        {t('收益')}: {renderQuota(invite.aff_history_quota || 0)}
      </Tag>
      <Tag color='white' shape='circle' className='!text-xs'>
        {invite.inviter_id
          ? `${t('邀请人')}: ${invite.inviter_id}`
          : t('无邀请人')}
      </Tag>
    </Space>
  );
};

const AgentUsers = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [userCount, setUserCount] = useState(0);
  const [keyword, setKeyword] = useState('');

  const loadUsers = useCallback(async (page, size, search) => {
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
  }, []);

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

  const handlePageChange = (page) => setActivePage(page);
  const handlePageSizeChange = (size) => {
    setPageSize(size);
    setActivePage(1);
  };

  const columns = useMemo(
    () => [
      {
        title: 'ID',
        dataIndex: 'id',
        width: 80,
      },
      {
        title: t('昵称'),
        dataIndex: 'nickname',
        render: (text) => text || '-',
      },
      {
        title: t('状态'),
        key: 'status',
        render: (text, record) => renderStatistics(record, t),
      },
      {
        title: t('剩余额度/总额度'),
        key: 'quota_usage',
        render: (text, record) => renderQuotaUsage(record, t),
      },
      {
        title: t('分组'),
        dataIndex: 'group',
        render: (text) => renderGroup(text ?? ''),
      },
      {
        title: t('角色'),
        dataIndex: 'role',
        render: (text) => renderRole(text, t),
      },
      {
        title: t('邀请信息'),
        key: 'invite',
        render: (text, record) => renderInviteInfo(record, t),
      },
      {
        title: t('注册时间'),
        dataIndex: 'registered_at',
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
    ],
    [t],
  );

  const descriptionArea = (
    <div className='flex items-center text-blue-500'>
      <IconUserGroup className='mr-2' />
      <Text>{t('邀请用户')}</Text>
    </div>
  );

  const actionsArea = (
    <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
      <div />
      <Space>
        <Input
          prefix={<IconSearch />}
          placeholder={t('用户ID/用户名/显示名')}
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
          total: userCount,
          onPageChange: handlePageChange,
          onPageSizeChange: handlePageSizeChange,
          isMobile,
          t,
        })}
        t={t}
      >
        <CardTable
          columns={columns}
          dataSource={users}
          loading={loading}
          rowKey='id'
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
              description={t('暂无数据')}
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

export default AgentUsers;
