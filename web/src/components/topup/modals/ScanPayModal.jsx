import React from 'react';
import { Button, Modal } from '@douyinfe/semi-ui';
import { QRCodeSVG } from 'qrcode.react';

const ScanPayModal = ({
  visible,
  title,
  qrCode,
  instruction,
  orderId,
  onCancel,
  onCheckPaid,
  checking = false,
  checkButtonText,
  orderLabel = 'ID',
}) => {
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
        <div className='rounded-xl border border-gray-200 bg-white p-4'>
          {qrCode ? (
            <QRCodeSVG value={qrCode} size={320} level='H' includeMargin={false} />
          ) : (
            <div className='flex h-80 w-80 items-center justify-center'>
              <div className='h-10 w-10 animate-spin rounded-full border-b-2 border-indigo-500' />
            </div>
          )}
        </div>
        <p className='mt-5 text-center text-lg leading-8 text-gray-700'>
          {instruction}
        </p>
        {orderId && (
          <p className='mt-3 text-sm text-gray-500'>
            {orderLabel}: <span className='font-mono'>{orderId}</span>
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
