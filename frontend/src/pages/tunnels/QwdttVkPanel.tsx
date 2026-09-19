// Copyright (c) 2025 LucX-UI Project / samur005 fork.
// Native VK hash generator UI for qWDTT (see CREDITS.md / WDTT provenance).
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Input, Space, Tag, Typography, message } from 'antd';

import { tunnelsApi } from '@/api/tunnels';

type Props = {
  existingHashes: string;
  onHashes: (hashes: string) => void;
  onInvalidate?: () => void;
};

export function QwdttVkPanel({ existingHashes, onHashes, onInvalidate }: Props) {
  const { t } = useTranslation();
  const [messageApi, ctx] = message.useMessage();
  const [vkCookies, setVkCookies] = useState('');
  const [vkHint, setVkHint] = useState('');
  const [vkOk, setVkOk] = useState(false);
  const [vkExpired, setVkExpired] = useState(false);
  const [vkPresent, setVkPresent] = useState(false);
  const [vkCallId, setVkCallId] = useState('');
  const [vkBusy, setVkBusy] = useState(false);

  const refreshVkStatus = async () => {
    const res = await tunnelsApi.vkStatus();
    if (!res.success || !res.obj) return;
    const o = res.obj as Record<string, unknown>;
    setVkOk(Boolean(o.cookies_ok));
    setVkExpired(Boolean(o.cookies_expired));
    setVkPresent(Boolean(o.cookies_present));
    setVkHint(String(o.cookies_hint ?? ''));
    if (typeof o.cookies_text === 'string' && o.cookies_text) {
      setVkCookies(o.cookies_text);
    }
    const sess = o.session as { call_id?: string } | undefined;
    setVkCallId(sess?.call_id ?? '');
  };

  useEffect(() => {
    void refreshVkStatus();
  }, []);

  return (
    <Card
      size="small"
      type="inner"
      title={t('pages.tunnels.qwdtt.vk.title', 'VK hash generator')}
      style={{ marginBottom: 16 }}
      extra={
        <Tag color={vkOk ? 'green' : vkExpired ? 'red' : vkPresent ? 'orange' : 'default'}>
          {vkOk
            ? t('pages.tunnels.qwdtt.vk.statusOk', 'Cookies OK')
            : vkExpired
              ? t('pages.tunnels.qwdtt.vk.statusExpired', 'Cookies expired')
              : t('pages.tunnels.qwdtt.vk.statusMissing', 'Cookies not set')}
        </Tag>
      }
    >
      {ctx}
      <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
        {t(
          'pages.tunnels.qwdtt.vk.hint',
          'Paste remixsid (or full Cookie header) from a logged-in vk.com / vk.ru session. Generate creates a live call and fills VkHashes.',
        )}
      </Typography.Paragraph>
      {vkHint ? (
        <Typography.Paragraph type={vkExpired ? 'danger' : 'secondary'} style={{ fontSize: 12 }}>
          {vkHint}
        </Typography.Paragraph>
      ) : null}
      <Typography.Text type="secondary">
        {t('pages.tunnels.qwdtt.vk.cookies', 'VK cookies / remixsid')}
      </Typography.Text>
      <Input.TextArea
        rows={3}
        value={vkCookies}
        onChange={(e) => setVkCookies(e.target.value)}
        placeholder={t(
          'pages.tunnels.qwdtt.vk.cookiesPlaceholder',
          'remixsid=VALUE   or  name=value; …',
        )}
        style={{ marginTop: 4, marginBottom: 8 }}
      />
      <Space wrap>
        <Button
          disabled={vkBusy || !vkCookies.trim()}
          loading={vkBusy}
          onClick={() => {
            void (async () => {
              setVkBusy(true);
              try {
                const res = await tunnelsApi.vkSaveCookies(vkCookies.trim());
                if (res.success) {
                  messageApi.success(t('pages.tunnels.qwdtt.vk.savedCookies', 'Cookies saved'));
                  await refreshVkStatus();
                } else if (res.msg) messageApi.error(res.msg);
              } finally {
                setVkBusy(false);
              }
            })();
          }}
        >
          {t('pages.tunnels.qwdtt.vk.saveCookies', 'Save cookies')}
        </Button>
        <Button
          disabled={vkBusy}
          onClick={() => {
            void (async () => {
              setVkBusy(true);
              try {
                const res = await tunnelsApi.vkClearCookies();
                if (res.success) {
                  setVkCookies('');
                  messageApi.success(t('pages.tunnels.qwdtt.vk.clearedCookies', 'Cookies cleared'));
                  await refreshVkStatus();
                } else if (res.msg) messageApi.error(res.msg);
              } finally {
                setVkBusy(false);
              }
            })();
          }}
        >
          {t('pages.tunnels.qwdtt.vk.clearCookies', 'Clear')}
        </Button>
        <Button
          type="primary"
          disabled={vkBusy}
          loading={vkBusy}
          onClick={() => {
            void (async () => {
              setVkBusy(true);
              try {
                const res = await tunnelsApi.vkCreate({ apply: true, existing: existingHashes });
                if (res.success && res.obj) {
                  const hash = String((res.obj as Record<string, unknown>).vk_hash ?? '');
                  if (hash) onHashes(hash);
                  messageApi.success(t('pages.tunnels.qwdtt.vk.generated', 'vk_hash generated'));
                  await refreshVkStatus();
                  onInvalidate?.();
                } else if (res.msg) messageApi.error(res.msg);
              } finally {
                setVkBusy(false);
              }
            })();
          }}
        >
          {t('pages.tunnels.qwdtt.vk.generate', 'Generate vk_hash')}
        </Button>
        <Button
          danger
          disabled={vkBusy || !vkCallId}
          onClick={() => {
            void (async () => {
              setVkBusy(true);
              try {
                const res = await tunnelsApi.vkStop(vkCallId);
                if (res.success) {
                  messageApi.success(t('pages.tunnels.qwdtt.vk.stopped', 'Call stop requested'));
                  await refreshVkStatus();
                } else if (res.msg) messageApi.error(res.msg);
              } finally {
                setVkBusy(false);
              }
            })();
          }}
        >
          {t('pages.tunnels.qwdtt.vk.stop', 'Stop call')}
        </Button>
      </Space>
    </Card>
  );
}
