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
  showSuccess,
  timestamp2string,
  renderGroupOption,
  renderQuotaWithPrompt,
  getModelCategories,
  selectFilter,
} from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import {
  Button,
  SideSheet,
  Space,
  Spin,
  Typography,
  Card,
  Tag,
  Avatar,
  Form,
  Col,
  Row,
} from '@douyinfe/semi-ui';
import {
  IconCreditCard,
  IconLink,
  IconSave,
  IconClose,
  IconKey,
  IconDelete,
  IconInfoCircle,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { StatusContext } from '../../../../context/Status';
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';

const { Text, Title } = Typography;

const SortableGroupItem = ({ group, index, groupMetaMap, onRemove, t }) => {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: group });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
    zIndex: isDragging ? 999 : 'auto',
  };

  const ratio = groupMetaMap[group]?.ratio;
  const ratioColor =
    ratio == null ? undefined : ratio >= 1 ? '#f93920' : '#00b42a';

  return (
    <div
      ref={setNodeRef}
      style={style}
      className='flex items-center gap-2 rounded-xl border border-gray-100 bg-white px-3 py-2.5'
    >
      <span
        {...attributes}
        {...listeners}
        className='cursor-grab text-gray-400 hover:text-gray-600 select-none'
        style={{ fontSize: 16, lineHeight: 1 }}
      >
        ⠿
      </span>
      <Tag color='blue' shape='circle' size='small'>
        {t('优先级')} {index + 1}
      </Tag>
      <Text strong style={{ fontSize: 13 }}>
        {group}
      </Text>
      {groupMetaMap[group]?.label && (
        <Text type='tertiary' size='small'>
          {groupMetaMap[group].label}
        </Text>
      )}
      {ratio != null && (
        <span
          style={{
            marginLeft: 'auto',
            fontSize: 12,
            fontWeight: 600,
            color: ratioColor,
            background: ratio >= 1 ? '#fff1f0' : '#f6ffed',
            border: `1px solid ${ratio >= 1 ? '#ffa39e' : '#b7eb8f'}`,
            borderRadius: 999,
            padding: '1px 10px',
            whiteSpace: 'nowrap',
          }}
        >
          {ratio}
          {t('倍')}
        </span>
      )}
      <Button
        theme='borderless'
        type='tertiary'
        icon={<IconDelete />}
        size='small'
        style={{ marginLeft: ratio != null ? 0 : 'auto', color: '#f93920' }}
        onClick={() => onRemove(index)}
      />
    </div>
  );
};

const EditTokenModal = (props) => {
  const { t } = useTranslation();
  const [statusState] = useContext(StatusContext);
  const [loading, setLoading] = useState(false);
  const isMobile = useIsMobile();
  const formApiRef = useRef(null);
  const [models, setModels] = useState([]);
  const [groups, setGroups] = useState([]);

  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  );

  const handleDragEnd = (event) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const currentGroups = parseGroupValue(formApiRef.current?.getValue('group'));
    const oldIndex = currentGroups.indexOf(active.id);
    const newIndex = currentGroups.indexOf(over.id);
    if (oldIndex !== -1 && newIndex !== -1) {
      setSelectedGroups(arrayMove(currentGroups, oldIndex, newIndex));
    }
  };
  const isEdit = props.editingToken.id !== undefined;
  const parseGroupValue = (groupValue) => {
    if (!groupValue) {
      return [];
    }
    const groupList = Array.isArray(groupValue)
      ? groupValue
      : `${groupValue}`.split(',');
    const seen = new Set();
    return groupList
      .map((group) => `${group}`.trim())
      .filter((group) => {
        if (!group || seen.has(group)) {
          return false;
        }
        seen.add(group);
        return true;
      });
  };
  const groupMetaMap = groups.reduce((acc, item) => {
    acc[item.value] = item;
    return acc;
  }, {});

  const getInitValues = () => ({
    name: '',
    remain_quota: 0,
    expired_time: -1,
    unlimited_quota: true,
    model_limits_enabled: false,
    model_limits: [],
    allow_ips: '',
    group: [],
    cross_group_retry: false,
    tokenCount: 1,
  });

  const handleCancel = () => {
    props.handleClose();
  };

  const setExpiredTime = (month, day, hour, minute) => {
    let now = new Date();
    let timestamp = now.getTime() / 1000;
    let seconds = month * 30 * 24 * 60 * 60;
    seconds += day * 24 * 60 * 60;
    seconds += hour * 60 * 60;
    seconds += minute * 60;
    if (!formApiRef.current) return;
    if (seconds !== 0) {
      timestamp += seconds;
      formApiRef.current.setValue('expired_time', timestamp2string(timestamp));
    } else {
      formApiRef.current.setValue('expired_time', -1);
    }
  };

  const setSelectedGroups = (nextGroups) => {
    if (!formApiRef.current) return;
    formApiRef.current.setValue('group', parseGroupValue(nextGroups));
  };

  const handleGroupSelectionChange = (nextGroups, currentGroups = []) => {
    let normalizedGroups = parseGroupValue(nextGroups);
    if (normalizedGroups.includes('auto') && normalizedGroups.length > 1) {
      showError(t('auto 分组不能与其他分组同时选择'));
      if (currentGroups.includes('auto')) {
        normalizedGroups = normalizedGroups.filter((group) => group !== 'auto');
      } else {
        normalizedGroups = ['auto'];
      }
    }
    setSelectedGroups(normalizedGroups);
    if (normalizedGroups.length > 1) {
      formApiRef.current?.setValue('cross_group_retry', true);
    }
  };

  const removeGroup = (index) => {
    const currentGroups = parseGroupValue(
      formApiRef.current?.getValue('group'),
    );
    setSelectedGroups(
      currentGroups.filter((_, currentIndex) => currentIndex !== index),
    );
  };

  const loadModels = async () => {
    let res = await API.get(`/api/user/models`);
    const { success, message, data } = res.data;
    if (success) {
      const categories = getModelCategories(t);
      let localModelOptions = data.map((model) => {
        let icon = null;
        for (const [key, category] of Object.entries(categories)) {
          if (key !== 'all' && category.filter({ model_name: model })) {
            icon = category.icon;
            break;
          }
        }
        return {
          label: (
            <span className='flex items-center gap-1'>
              {icon}
              {model}
            </span>
          ),
          value: model,
        };
      });
      setModels(localModelOptions);
    } else {
      showError(t(message));
    }
  };

  const loadGroups = async () => {
    let res = await API.get(`/api/user/self/groups`);
    const { success, message, data } = res.data;
    if (success) {
      let localGroupOptions = Object.entries(data).map(([group, info]) => ({
        label: info.desc,
        value: group,
        ratio: info.ratio,
      }));
      if (statusState?.status?.default_use_auto_group) {
        if (localGroupOptions.some((group) => group.value === 'auto')) {
          localGroupOptions.sort((a, b) => (a.value === 'auto' ? -1 : 1));
        }
      }
      setGroups(localGroupOptions);
      // if (statusState?.status?.default_use_auto_group && formApiRef.current) {
      //   formApiRef.current.setValue('group', 'auto');
      // }
    } else {
      showError(t(message));
    }
  };

  const loadToken = async () => {
    setLoading(true);
    let res = await API.get(`/api/token/${props.editingToken.id}`);
    const { success, message, data } = res.data;
    if (success) {
      if (data.expired_time !== -1) {
        data.expired_time = timestamp2string(data.expired_time);
      }
      if (data.model_limits !== '') {
        data.model_limits = data.model_limits.split(',');
      } else {
        data.model_limits = [];
      }
      data.group = parseGroupValue(data.group);
      if (formApiRef.current) {
        formApiRef.current.setValues({ ...getInitValues(), ...data });
      }
    } else {
      showError(message);
    }
    setLoading(false);
  };

  useEffect(() => {
    if (formApiRef.current) {
      if (!isEdit) {
        formApiRef.current.setValues(getInitValues());
      }
    }
    loadModels();
    loadGroups();
  }, [props.editingToken.id]);

  useEffect(() => {
    if (props.visiable) {
      if (isEdit) {
        loadToken();
      } else {
        formApiRef.current?.setValues(getInitValues());
      }
    } else {
      formApiRef.current?.reset();
    }
  }, [props.visiable, props.editingToken.id]);

  const generateRandomSuffix = () => {
    const characters =
      'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
    let result = '';
    for (let i = 0; i < 6; i++) {
      result += characters.charAt(
        Math.floor(Math.random() * characters.length),
      );
    }
    return result;
  };

  const submit = async (values) => {
    setLoading(true);
    if (isEdit) {
      let { tokenCount: _tc, ...localInputs } = values;
      const selectedGroups = parseGroupValue(localInputs.group);
      localInputs.remain_quota = parseInt(localInputs.remain_quota);
      if (localInputs.expired_time !== -1) {
        let time = Date.parse(localInputs.expired_time);
        if (isNaN(time)) {
          showError(t('过期时间格式错误！'));
          setLoading(false);
          return;
        }
        localInputs.expired_time = Math.ceil(time / 1000);
      }
      localInputs.group = selectedGroups.join(',');
      if (selectedGroups.length > 1) {
        localInputs.cross_group_retry = true;
      }
      localInputs.model_limits = localInputs.model_limits.join(',');
      localInputs.model_limits_enabled = localInputs.model_limits.length > 0;
      let res = await API.put(`/api/token/`, {
        ...localInputs,
        id: parseInt(props.editingToken.id),
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('令牌更新成功！'));
        props.refresh();
        props.handleClose();
      } else {
        showError(t(message));
      }
    } else {
      const count = parseInt(values.tokenCount, 10) || 1;
      let successCount = 0;
      for (let i = 0; i < count; i++) {
        let { tokenCount: _tc, ...localInputs } = values;
        const selectedGroups = parseGroupValue(localInputs.group);
        const baseName =
          values.name.trim() === '' ? 'default' : values.name.trim();
        if (i !== 0 || values.name.trim() === '') {
          localInputs.name = `${baseName}-${generateRandomSuffix()}`;
        } else {
          localInputs.name = baseName;
        }
        localInputs.remain_quota = parseInt(localInputs.remain_quota);

        if (localInputs.expired_time !== -1) {
          let time = Date.parse(localInputs.expired_time);
          if (isNaN(time)) {
            showError(t('过期时间格式错误！'));
            setLoading(false);
            break;
          }
          localInputs.expired_time = Math.ceil(time / 1000);
        }
        localInputs.group = selectedGroups.join(',');
        if (selectedGroups.length > 1) {
          localInputs.cross_group_retry = true;
        }
        localInputs.model_limits = localInputs.model_limits.join(',');
        localInputs.model_limits_enabled = localInputs.model_limits.length > 0;
        let res = await API.post(`/api/token/`, localInputs);
        const { success, message } = res.data;
        if (success) {
          successCount++;
        } else {
          showError(t(message));
          break;
        }
      }
      if (successCount > 0) {
        showSuccess(t('令牌创建成功，请在列表页面点击复制获取令牌！'));
        props.refresh();
        props.handleClose();
      }
    }
    setLoading(false);
    formApiRef.current?.setValues(getInitValues());
  };

  return (
    <SideSheet
      placement={isEdit ? 'right' : 'left'}
      title={
        <Space>
          {isEdit ? (
            <Tag color='blue' shape='circle'>
              {t('更新')}
            </Tag>
          ) : (
            <Tag color='green' shape='circle'>
              {t('新建')}
            </Tag>
          )}
          <Title heading={4} className='m-0'>
            {isEdit ? t('更新令牌信息') : t('创建新的令牌')}
          </Title>
        </Space>
      }
      bodyStyle={{ padding: '0' }}
      visible={props.visiable}
      width={isMobile ? '100%' : 600}
      footer={
        <div className='flex justify-end bg-white'>
          <Space>
            <Button
              theme='solid'
              className='!rounded-lg'
              onClick={() => formApiRef.current?.submitForm()}
              icon={<IconSave />}
              loading={loading}
            >
              {t('提交')}
            </Button>
            <Button
              theme='light'
              className='!rounded-lg'
              type='primary'
              onClick={handleCancel}
              icon={<IconClose />}
            >
              {t('取消')}
            </Button>
          </Space>
        </div>
      }
      closeIcon={null}
      onCancel={() => handleCancel()}
    >
      <Spin spinning={loading}>
        <Form
          key={isEdit ? 'edit' : 'new'}
          initValues={getInitValues()}
          getFormApi={(api) => (formApiRef.current = api)}
          onSubmit={submit}
        >
          {({ values }) => {
            const selectedGroups = parseGroupValue(values.group);
            const isAutoSelected =
              selectedGroups.length === 1 && selectedGroups[0] === 'auto';
            const hasPriorityGroups = selectedGroups.length > 1;

            return (
              <div className='p-2'>
                {/* 基本信息 */}
                <Card className='!rounded-2xl shadow-sm border-0'>
                  <div className='flex items-center mb-2'>
                    <Avatar
                      size='small'
                      color='blue'
                      className='mr-2 shadow-md'
                    >
                      <IconKey size={16} />
                    </Avatar>
                    <div>
                      <Text className='text-lg font-medium'>
                        {t('基本信息')}
                      </Text>
                      <div className='text-xs text-gray-600'>
                        {t('设置令牌的基本信息')}
                      </div>
                    </div>
                  </div>
                  <Row gutter={12}>
                    <Col span={24}>
                      <Form.Input
                        field='name'
                        label={t('名称')}
                        placeholder={t('请输入名称')}
                        rules={[{ required: true, message: t('请输入名称') }]}
                        showClear
                      />
                    </Col>
                    <Col span={24}>
                      {groups.length > 0 ? (
                        <Form.Select
                          field='group'
                          label={t('令牌分组')}
                          placeholder={t(
                            '可选择多个分组，顺序越靠前优先级越高',
                          )}
                          multiple
                          maxTagCount={3}
                          optionList={groups}
                          renderOptionItem={renderGroupOption}
                          onChange={(nextGroups) =>
                            handleGroupSelectionChange(
                              nextGroups,
                              selectedGroups,
                            )
                          }
                          showClear
                          style={{ width: '100%' }}
                        />
                      ) : (
                        <Form.Select
                          placeholder={t('管理员未设置用户可选分组')}
                          disabled
                          label={t('令牌分组')}
                          style={{ width: '100%' }}
                        />
                      )}
                    </Col>
                    {selectedGroups.length > 0 && (
                      <Col span={24}>
                        <div
                          style={{
                            borderRadius: 10,
                            background: '#eff6ff',
                            border: '1px solid #bfdbfe',
                            padding: '10px 14px',
                            display: 'flex',
                            gap: 8,
                            alignItems: 'flex-start',
                          }}
                        >
                          <IconInfoCircle
                            style={{ color: '#3b82f6', marginTop: 2, flexShrink: 0 }}
                          />
                          <div style={{ fontSize: 12, color: '#1d4ed8', lineHeight: 1.7 }}>
                            <div>• {t('选择顺序决定分组优先级（第一个为主分组）')}</div>
                            <div>• {t('系统会按优先级顺序尝试各分组渠道')}</div>
                            <div>• {t('建议选择2-3个分组以确保服务稳定性')}</div>
                          </div>
                        </div>
                      </Col>
                    )}
                    {hasPriorityGroups && (
                      <Col span={24}>
                        <Card
                          className='!rounded-xl border border-gray-200'
                          bodyStyle={{ padding: 12 }}
                        >
                          <div className='mb-3 flex items-center justify-between'>
                            <Text strong>{t('当前优先级顺序')}</Text>
                            <Text type='tertiary' size='small'>
                              {t('拖拽调整顺序')}
                            </Text>
                          </div>
                          <DndContext
                            sensors={sensors}
                            collisionDetection={closestCenter}
                            onDragEnd={handleDragEnd}
                          >
                            <SortableContext
                              items={selectedGroups}
                              strategy={verticalListSortingStrategy}
                            >
                              <div className='flex flex-col gap-2'>
                                {selectedGroups.map((group, index) => (
                                  <SortableGroupItem
                                    key={group}
                                    group={group}
                                    index={index}
                                    groupMetaMap={groupMetaMap}
                                    onRemove={removeGroup}
                                    t={t}
                                  />
                                ))}
                              </div>
                            </SortableContext>
                          </DndContext>
                        </Card>
                      </Col>
                    )}
                    <Col
                      span={24}
                      style={{
                        display: isAutoSelected ? 'block' : 'none',
                      }}
                    >
                      <Form.Switch
                        field='cross_group_retry'
                        label={t('跨分组重试')}
                        size='default'
                        extraText={t(
                          '开启后，当前分组渠道失败时会按顺序尝试下一个分组的渠道',
                        )}
                      />
                    </Col>
                    {hasPriorityGroups && (
                      <Col span={24}>
                        <div className='rounded-xl border border-emerald-100 bg-emerald-50 px-4 py-3 text-xs text-emerald-700'>
                          {t(
                            '多分组令牌默认会按优先级自动降级，不需要额外开启跨分组重试。',
                          )}
                        </div>
                      </Col>
                    )}
                    <Col xs={24} sm={24} md={24} lg={10} xl={10}>
                      <Form.DatePicker
                        field='expired_time'
                        label={t('过期时间')}
                        type='dateTime'
                        placeholder={t('请选择过期时间')}
                        rules={[
                          { required: true, message: t('请选择过期时间') },
                          {
                            validator: (rule, value) => {
                              // 允许 -1 表示永不过期，也允许空值在必填校验时被拦截
                              if (value === -1 || !value)
                                return Promise.resolve();
                              const time = Date.parse(value);
                              if (isNaN(time)) {
                                return Promise.reject(t('过期时间格式错误！'));
                              }
                              if (time <= Date.now()) {
                                return Promise.reject(
                                  t('过期时间不能早于当前时间！'),
                                );
                              }
                              return Promise.resolve();
                            },
                          },
                        ]}
                        showClear
                        style={{ width: '100%' }}
                      />
                    </Col>
                    <Col xs={24} sm={24} md={24} lg={14} xl={14}>
                      <Form.Slot label={t('过期时间快捷设置')}>
                        <Space wrap>
                          <Button
                            theme='light'
                            type='primary'
                            onClick={() => setExpiredTime(0, 0, 0, 0)}
                          >
                            {t('永不过期')}
                          </Button>
                          <Button
                            theme='light'
                            type='tertiary'
                            onClick={() => setExpiredTime(1, 0, 0, 0)}
                          >
                            {t('一个月')}
                          </Button>
                          <Button
                            theme='light'
                            type='tertiary'
                            onClick={() => setExpiredTime(0, 1, 0, 0)}
                          >
                            {t('一天')}
                          </Button>
                          <Button
                            theme='light'
                            type='tertiary'
                            onClick={() => setExpiredTime(0, 0, 1, 0)}
                          >
                            {t('一小时')}
                          </Button>
                        </Space>
                      </Form.Slot>
                    </Col>
                    {!isEdit && (
                      <Col span={24}>
                        <Form.InputNumber
                          field='tokenCount'
                          label={t('新建数量')}
                          min={1}
                          extraText={t('批量创建时会在名称后自动添加随机后缀')}
                          rules={[
                            { required: true, message: t('请输入新建数量') },
                          ]}
                          style={{ width: '100%' }}
                        />
                      </Col>
                    )}
                  </Row>
                </Card>

                {/* 额度设置 */}
                <Card className='!rounded-2xl shadow-sm border-0'>
                  <div className='flex items-center mb-2'>
                    <Avatar
                      size='small'
                      color='green'
                      className='mr-2 shadow-md'
                    >
                      <IconCreditCard size={16} />
                    </Avatar>
                    <div>
                      <Text className='text-lg font-medium'>
                        {t('额度设置')}
                      </Text>
                      <div className='text-xs text-gray-600'>
                        {t('设置令牌可用额度和数量')}
                      </div>
                    </div>
                  </div>
                  <Row gutter={12}>
                    <Col span={24}>
                      <Form.AutoComplete
                        field='remain_quota'
                        label={t('额度')}
                        placeholder={t('请输入额度')}
                        type='number'
                        disabled={values.unlimited_quota}
                        extraText={renderQuotaWithPrompt(values.remain_quota)}
                        rules={
                          values.unlimited_quota
                            ? []
                            : [{ required: true, message: t('请输入额度') }]
                        }
                        data={[
                          { value: 500000, label: '1$' },
                          { value: 5000000, label: '10$' },
                          { value: 25000000, label: '50$' },
                          { value: 50000000, label: '100$' },
                          { value: 250000000, label: '500$' },
                          { value: 500000000, label: '1000$' },
                        ]}
                      />
                    </Col>
                    <Col span={24}>
                      <Form.Switch
                        field='unlimited_quota'
                        label={t('无限额度')}
                        size='default'
                        extraText={t(
                          '令牌的额度仅用于限制令牌本身的最大额度使用量，实际的使用受到账户的剩余额度限制',
                        )}
                      />
                    </Col>
                  </Row>
                </Card>

                {/* 访问限制 */}
                <Card className='!rounded-2xl shadow-sm border-0'>
                  <div className='flex items-center mb-2'>
                    <Avatar
                      size='small'
                      color='purple'
                      className='mr-2 shadow-md'
                    >
                      <IconLink size={16} />
                    </Avatar>
                    <div>
                      <Text className='text-lg font-medium'>
                        {t('访问限制')}
                      </Text>
                      <div className='text-xs text-gray-600'>
                        {t('设置令牌的访问限制')}
                      </div>
                    </div>
                  </div>
                  <Row gutter={12}>
                    <Col span={24}>
                      <Form.Select
                        field='model_limits'
                        label={t('模型限制列表')}
                        placeholder={t(
                          '请选择该令牌支持的模型，留空支持所有模型',
                        )}
                        multiple
                        optionList={models}
                        extraText={t('非必要，不建议启用模型限制')}
                        filter={selectFilter}
                        autoClearSearchValue={false}
                        searchPosition='dropdown'
                        showClear
                        style={{ width: '100%' }}
                      />
                    </Col>
                    <Col span={24}>
                      <Form.TextArea
                        field='allow_ips'
                        label={t('IP白名单（支持CIDR表达式）')}
                        placeholder={t('允许的IP，一行一个，不填写则不限制')}
                        autosize
                        rows={1}
                        extraText={t(
                          '请勿过度信任此功能，IP可能被伪造，请配合nginx和cdn等网关使用',
                        )}
                        showClear
                        style={{ width: '100%' }}
                      />
                    </Col>
                  </Row>
                </Card>
              </div>
            );
          }}
        </Form>
      </Spin>
    </SideSheet>
  );
};

export default EditTokenModal;
