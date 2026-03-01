import React, { useEffect, useState } from 'react';
import { Button, Modal } from '@douyinfe/semi-ui';
import { QRCodeSVG } from 'qrcode.react';

const ScanPayModal = ({
  visible,
  title,
  qrCode,
  instruction,
  orderId,
  expireTime = 0,
  expired = false,
  refreshing = false,
  onRefresh,
  onCancel,
  onCheckPaid,
  checking = false,
  checkButtonText,
  orderLabel = 'ID',
  expireLabel = '过期时间',
  countdownLabel = '剩余时间',
  expiredText = '',
  refreshText = '',
  refreshingText = '',
}) => {
  const [remainingSeconds, setRemainingSeconds] = useState(0);

  const formatExpireTime = (unixSeconds) => {
    if (!unixSeconds || Number(unixSeconds) <= 0) return '';
    const date = new Date(Number(unixSeconds) * 1000);
    if (Number.isNaN(date.getTime())) return '';
    const pad = (n) => String(n).padStart(2, '0');
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(
      date.getDate(),
    )} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(
      date.getSeconds(),
    )}`;
  };

  const formatRemaining = (seconds) => {
    if (!seconds || seconds <= 0) return '';
    const total = Math.max(0, Number(seconds));
    const hours = Math.floor(total / 3600);
    const minutes = Math.floor((total % 3600) / 60);
    const secs = total % 60;
    const pad = (n) => String(n).padStart(2, '0');
    if (hours > 0) {
      return `${pad(hours)}:${pad(minutes)}:${pad(secs)}`;
    }
    return `${pad(minutes)}:${pad(secs)}`;
  };

  useEffect(() => {
    if (!visible || !expireTime || Number(expireTime) <= 0) {
      setRemainingSeconds(0);
      return;
    }
    const tick = () => {
      const nowSeconds = Math.floor(Date.now() / 1000);
      const left = Math.max(0, Number(expireTime) - nowSeconds);
      setRemainingSeconds(left);
    };
    tick();
    const timer = setInterval(tick, 1000);
    return () => clearInterval(timer);
  }, [visible, expireTime]);

  const expireTimeText = formatExpireTime(expireTime);
  const remainTimeText = formatRemaining(remainingSeconds);
  const localExpired = Boolean(
    expired || (expireTime > 0 && Date.now() >= Number(expireTime) * 1000),
  );

  return (
    <Modal
      title={title}
      visible={visible}
      onCancel={onCancel}
      footer={null}
      centered
      size='small'
      bodyStyle={{ padding: 0 }}
    >
      <div className='flex flex-col items-center px-8 py-8'>
        <div className='relative rounded-xl border border-gray-200 bg-white p-4'>
          {qrCode ? (
            <QRCodeSVG value={qrCode} size={320} level='H' includeMargin={false} />
          ) : (
            <div className='flex h-80 w-80 items-center justify-center'>
              <div className='h-10 w-10 animate-spin rounded-full border-b-2 border-indigo-500' />
            </div>
          )}
          {localExpired && (
            <div
              className='absolute inset-0 flex items-center justify-center rounded-xl'
              style={{ backgroundColor: 'rgba(255, 255, 255, 0.94)' }}
            >
              <div className='flex flex-col items-center rounded-lg bg-white px-8 py-6 shadow-md'>
                <p className='text-3xl font-semibold text-gray-800'>{expiredText}</p>
                <Button
                  type='primary'
                  size='default'
                  className='mt-5 min-w-[132px]'
                  onClick={onRefresh}
                  loading={refreshing}
                  disabled={!onRefresh}
                >
                  {refreshing ? refreshingText : refreshText}
                </Button>
              </div>
            </div>
          )}
        </div>
        <p className='mt-5 text-center text-lg leading-8 text-gray-700'>
          {instruction}
        </p>
        {!localExpired && remainTimeText && (
          <p className='mt-2 text-sm text-gray-500'>
            {countdownLabel}: <span className='font-mono'>{remainTimeText}</span>
          </p>
        )}
        {orderId && (
          <p className='mt-3 text-sm text-gray-500'>
            {orderLabel}: <span className='font-mono'>{orderId}</span>
          </p>
        )}
        {expireTimeText && (
          <p className='mt-2 text-sm text-gray-500'>
            {expireLabel}: <span className='font-mono'>{expireTimeText}</span>
          </p>
        )}
        <Button
          type='primary'
          size='large'
          loading={checking}
          onClick={onCheckPaid}
          className='mt-6 min-w-[180px]'
        >
          {checkButtonText}
        </Button>
      </div>
    </Modal>
  );
};

export default ScanPayModal;
