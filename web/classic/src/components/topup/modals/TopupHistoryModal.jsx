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
import React, { useState, useEffect, useMemo } from 'react';
import {
  Modal,
  Table,
  Badge,
  Typography,
  Toast,
  Empty,
  Button,
  Input,
  Tag,
  Spin,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { Coins, Trash2 } from 'lucide-react';
import { IconSearch, IconAlertTriangle } from '@douyinfe/semi-icons';
import { API, timestamp2string } from '../../../helpers';
import { isAdmin } from '../../../helpers/utils';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
const { Text, Title } = Typography;

// 状态映射配置
const STATUS_CONFIG = {
  success: { type: 'success', key: '成功' },
  pending: { type: 'warning', key: '待支付' },
  failed: { type: 'danger', key: '失败' },
  expired: { type: 'danger', key: '已过期' },
};

// 支付方式映射
const PAYMENT_METHOD_MAP = {
  stripe: 'Stripe',
  creem: 'Creem',
  waffo: 'Waffo',
  alipay: '支付宝',
  wxpay: '微信',
};

const TopupHistoryModal = ({ visible, onCancel, t }) => {
  const [loading, setLoading] = useState(false);
  const [topups, setTopups] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [keyword, setKeyword] = useState('');
  const isMobile = useIsMobile();

  // 清理未付订单相关状态
  const [previewing, setPreviewing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [previewItems, setPreviewItems] = useState([]);
  const [previewTotal, setPreviewTotal] = useState(0);

  const loadTopups = async (currentPage, currentPageSize) => {
    setLoading(true);
    try {
      const base = isAdmin() ? '/api/user/topup' : '/api/user/topup/self';
      const qs =
        `p=${currentPage}&page_size=${currentPageSize}` +
        (keyword ? `&keyword=${encodeURIComponent(keyword)}` : '');
      const endpoint = `${base}?${qs}`;
      const res = await API.get(endpoint);
      const { success, message, data } = res.data;
      if (success) {
        setTopups(data.items || []);
        setTotal(data.total || 0);
      } else {
        Toast.error({ content: message || t('加载失败') });
      }
    } catch (error) {
      Toast.error({ content: t('加载账单失败') });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (visible) {
      loadTopups(page, pageSize);
    }
  }, [visible, page, pageSize, keyword]);

  const handlePageChange = (currentPage) => {
    setPage(currentPage);
  };

  const handlePageSizeChange = (currentPageSize) => {
    setPageSize(currentPageSize);
    setPage(1);
  };

  const handleKeywordChange = (value) => {
    setKeyword(value);
    setPage(1);
  };

  // 管理员补单
  const handleAdminComplete = async (tradeNo) => {
    try {
      const res = await API.post('/api/user/topup/complete', {
        trade_no: tradeNo,
      });
      const { success, message } = res.data;
      if (success) {
        Toast.success({ content: t('补单成功') });
        await loadTopups(page, pageSize);
      } else {
        Toast.error({ content: message || t('补单失败') });
      }
    } catch (e) {
      Toast.error({ content: t('补单失败') });
    }
  };

  const confirmAdminComplete = (tradeNo) => {
    Modal.confirm({
      title: t('确认补单'),
      content: t('是否将该订单标记为成功并为用户入账？'),
      onOk: () => handleAdminComplete(tradeNo),
    });
  };

  // 管理员预览并清理超时未付订单
  const handlePreviewPending = async () => {
    setPreviewing(true);
    try {
      const res = await API.get('/api/user/topup/pending-preview?expire_hours=24');
      const { success, message, data } = res.data;
      if (success) {
        setPreviewItems(data.items || []);
        setPreviewTotal(data.total || 0);
        setPreviewVisible(true);
      } else {
        Toast.error({ content: message || t('加载预览失败') });
      }
    } catch (e) {
      Toast.error({ content: t('加载预览失败') });
    } finally {
      setPreviewing(false);
    }
  };

  const handleDeletePending = async () => {
    setDeleting(true);
    try {
      const res = await API.post('/api/user/topup/delete-pending', { expire_hours: 24 });
      const { success, message, data } = res.data;
      if (success) {
        Toast.success({
          content: t('删除 {{count}} 笔未付订单', { count: data?.deleted ?? 0 }),
        });
        setPreviewVisible(false);
        await loadTopups(page, pageSize);
      } else {
        Toast.error({ content: message || t('补单失败') });
      }
    } catch (e) {
      Toast.error({ content: t('补单失败') });
    } finally {
      setDeleting(false);
    }
  };

  // 渲染状态徽章
  const renderStatusBadge = (status) => {
    const config = STATUS_CONFIG[status] || { type: 'primary', key: status };
    return (
      <span className='flex items-center gap-2'>
        <Badge dot type={config.type} />
        <span>{t(config.key)}</span>
      </span>
    );
  };

  // 渲染支付方式
  const renderPaymentMethod = (pm) => {
    const displayName = PAYMENT_METHOD_MAP[pm];
    return <Text>{displayName ? t(displayName) : pm || '-'}</Text>;
  };

  const isSubscriptionTopup = (record) => {
    const tradeNo = (record?.trade_no || '').toLowerCase();
    return Number(record?.amount || 0) === 0 && tradeNo.startsWith('sub');
  };

  // 检查是否为管理员
  const userIsAdmin = useMemo(() => isAdmin(), []);

  const columns = useMemo(() => {
    const baseColumns = [
      ...(userIsAdmin
        ? [
            {
              title: t('用户ID'),
              dataIndex: 'user_id',
              key: 'user_id',
              render: (userId) => <Text>{userId ?? '-'}</Text>,
            },
          ]
        : []),
      {
        title: t('订单号'),
        dataIndex: 'trade_no',
        key: 'trade_no',
        render: (text) => <Text copyable>{text}</Text>,
      },
      {
        title: t('支付方式'),
        dataIndex: 'payment_method',
        key: 'payment_method',
        render: renderPaymentMethod,
      },
      {
        title: t('充值额度'),
        dataIndex: 'amount',
        key: 'amount',
        render: (amount, record) => {
          if (isSubscriptionTopup(record)) {
            return (
              <Tag color='purple' shape='circle' size='small'>
                {t('订阅套餐')}
              </Tag>
            );
          }
          return (
            <span className='flex items-center gap-1'>
              <Coins size={16} />
              <Text>{amount}</Text>
            </span>
          );
        },
      },
      {
        title: t('支付金额'),
        dataIndex: 'money',
        key: 'money',
        render: (money) => <Text type='danger'>¥{money.toFixed(2)}</Text>,
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        key: 'status',
        render: renderStatusBadge,
      },
    ];

    // 管理员才显示操作列
    if (userIsAdmin) {
      baseColumns.push({
        title: t('操作'),
        key: 'action',
        render: (_, record) => {
          const actions = [];
          if (record.status === 'pending') {
            actions.push(
              <Button
                key="complete"
                size='small'
                type='primary'
                theme='outline'
                onClick={() => confirmAdminComplete(record.trade_no)}
              >
                {t('补单')}
              </Button>
            );
          }
          return actions.length > 0 ? <>{actions}</> : null;
        },
      });
    }

    baseColumns.push({
      title: t('创建时间'),
      dataIndex: 'create_time',
      key: 'create_time',
      render: (time) => timestamp2string(time),
    });

    return baseColumns;
  }, [t, userIsAdmin]);

  return (
    <>
      <Modal
        title={t('充值账单')}
        visible={visible}
        onCancel={onCancel}
        footer={null}
        size={isMobile ? 'full-width' : 'large'}
      >
        <div className='mb-3 flex items-center gap-2'>
          <div className='flex-1'>
            <Input
              prefix={<IconSearch />}
              placeholder={t('订单号')}
              value={keyword}
              onChange={handleKeywordChange}
              showClear
            />
          </div>
          {/* 管理员：清理超时未付订单 */}
          {userIsAdmin && (
            <Button
              type='danger'
              theme='light'
              size='default'
              icon={<Trash2 size={14} />}
              loading={previewing}
              disabled={deleting || loading}
              onClick={handlePreviewPending}
              title={t('超过 24 小时未支付的订单将被永久删除')}
            >
              {previewing ? t('清理未付订单中...') : t('清理未付订单')}
            </Button>
          )}
        </div>
        <Table
          columns={columns}
          dataSource={topups}
          loading={loading}
          rowKey='id'
          pagination={{
            currentPage: page,
            pageSize: pageSize,
            total: total,
            showSizeChanger: true,
            pageSizeOpts: [10, 20, 50, 100],
            onPageChange: handlePageChange,
            onPageSizeChange: handlePageSizeChange,
          }}
          size='small'
          empty={
            <Empty
              image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
              darkModeImage={
                <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
              }
              description={t('暂无充值记录')}
              style={{ padding: 30 }}
            />
          }
        />
      </Modal>

      {/* 预览并确认删除弹窗 */}
      <Modal
        title={
          <span className='flex items-center gap-2'>
            <IconAlertTriangle style={{ color: 'var(--semi-color-danger)' }} />
            {t('清理未付订单')}
          </span>
        }
        visible={previewVisible}
        onCancel={() => !deleting && setPreviewVisible(false)}
        footer={
          <div className='flex justify-end gap-2'>
            <Button
              disabled={deleting}
              onClick={() => setPreviewVisible(false)}
            >
              {t('取消')}
            </Button>
            <Button
              type='danger'
              theme='solid'
              loading={deleting}
              disabled={previewTotal === 0}
              onClick={handleDeletePending}
            >
              {deleting
                ? t('清理未付订单中...')
                : t('删除 {{count}} 笔未付订单', { count: previewTotal })}
            </Button>
          </div>
        }
        size={isMobile ? 'full-width' : 'medium'}
      >
        <p className='mb-3 text-sm' style={{ color: 'var(--semi-color-text-1)' }}>
          {previewTotal > previewItems.length
            ? t('以下为将被删除的订单预览（最多显示 100 条）')
            : t('共 {{total}} 笔，确认删除？', { total: previewTotal })}
        </p>
        {previewItems.length === 0 ? (
          <Empty
            image={<IllustrationNoResult style={{ width: 120, height: 120 }} />}
            darkModeImage={
              <IllustrationNoResultDark style={{ width: 120, height: 120 }} />
            }
            description={t('暂无超时未支付订单')}
            style={{ padding: 20 }}
          />
        ) : (
          <Table
            size='small'
            dataSource={previewItems}
            rowKey='id'
            pagination={false}
            scroll={{ y: 320 }}
            columns={[
              {
                title: t('订单号'),
                dataIndex: 'trade_no',
                key: 'trade_no',
                render: (text) => <Text copyable ellipsis style={{ maxWidth: 160 }}>{text}</Text>,
              },
              ...(userIsAdmin
                ? [{
                    title: t('用户ID'),
                    dataIndex: 'user_id',
                    key: 'user_id',
                    width: 80,
                    render: (v) => <Text>{v ?? '-'}</Text>,
                  }]
                : []),
              {
                title: t('金额'),
                dataIndex: 'amount',
                key: 'amount',
                width: 80,
                render: (v) => <Text>${Number(v ?? 0).toFixed(2)}</Text>,
              },
              {
                title: t('创建时间'),
                dataIndex: 'create_time',
                key: 'create_time',
                render: (time) => timestamp2string(time),
              },
            ]}
          />
        )}
      </Modal>
    </>
  );
};

export default TopupHistoryModal;
