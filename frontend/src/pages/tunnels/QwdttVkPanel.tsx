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

  const applyVkStatus = (o: Record<string, unknown>) => {
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

  const refreshVkStatus = async () => {
    const res = await tunnelsApi.vkStatus();
    if (!res.success || !res.obj) return;
    applyVkStatus(res.obj as Record<string, unknown>);
  };

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const res = await tunnelsApi.vkStatus();
      if (cancelled || !res.success || !res.obj) return;
      applyVkStatus(res.obj as Record<string, unknown>);
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <Card
      size="small"
      type="inner"
      title={t('pages.tunnels.qwdtt.vk.title')}
      style={{ marginBottom: 16 }}
      extra={
        <Tag color={vkOk ? 'green' : vkExpired ? 'red' : vkPresent ? 'orange' : 'default'}>
          {vkOk
            ? t('pages.tunnels.qwdtt.vk.statusOk')
            : vkExpired
              ? t('pages.tunnels.qwdtt.vk.statusExpired')
              : t('pages.tunnels.qwdtt.vk.statusMissing')}
        </Tag>
      }
    >
      {ctx}
      <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
        {t('pages.tunnels.qwdtt.vk.hint')}
      </Typography.Paragraph>
      {vkHint ? (
        <Typography.Paragraph type={vkExpired ? 'danger' : 'secondary'} style={{ fontSize: 12 }}>
          {vkHint}
        </Typography.Paragraph>
      ) : null}
      <Typography.Text type="secondary">{t('pages.tunnels.qwdtt.vk.cookies')}</Typography.Text>
      <Input.TextArea
        rows={3}
        value={vkCookies}
        onChange={(e) => setVkCookies(e.target.value)}
        placeholder={t('pages.tunnels.qwdtt.vk.cookiesPlaceholder')}
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
                  messageApi.success(t('pages.tunnels.qwdtt.vk.savedCookies'));
                  await refreshVkStatus();
                } else if (res.msg) messageApi.error(res.msg);
              } finally {
                setVkBusy(false);
              }
            })();
          }}
        >
          {t('pages.tunnels.qwdtt.vk.saveCookies')}
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
                  messageApi.success(t('pages.tunnels.qwdtt.vk.clearedCookies'));
                  await refreshVkStatus();
                } else if (res.msg) messageApi.error(res.msg);
              } finally {
                setVkBusy(false);
              }
            })();
          }}
        >
          {t('pages.tunnels.qwdtt.vk.clearCookies')}
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
                  messageApi.success(t('pages.tunnels.qwdtt.vk.generated'));
                  await refreshVkStatus();
                  onInvalidate?.();
                } else if (res.msg) messageApi.error(res.msg);
              } finally {
                setVkBusy(false);
              }
            })();
          }}
        >
          {t('pages.tunnels.qwdtt.vk.generate')}
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
                  messageApi.success(t('pages.tunnels.qwdtt.vk.stopped'));
                  await refreshVkStatus();
                } else if (res.msg) messageApi.error(res.msg);
              } finally {
                setVkBusy(false);
              }
            })();
          }}
        >
          {t('pages.tunnels.qwdtt.vk.stop')}
        </Button>
      </Space>
    </Card>
  );
}
