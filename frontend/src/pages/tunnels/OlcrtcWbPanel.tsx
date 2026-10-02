// Copyright (c) 2025 LucX-UI Project / samur005 fork.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// WB Stream room generator for olcRTC (provider wbstream). Room API adapted
// from kulikov0/whitelist-bypass (MIT) — see CREDITS.md / docs/WBROOM.md.
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Card, Input, Select, Space, Tag, Typography, message } from 'antd';

import { tunnelsApi, type WbStatus } from '@/api/tunnels';

type Props = {
  /** Fill the room id into the surrounding form (target "form"). */
  onRoom?: (roomId: string) => void;
  onInvalidate?: () => void;
};

const FORM_TARGET = 0;

function fmtTime(sec?: number): string {
  if (!sec) return '';
  return new Date(sec * 1000).toLocaleString();
}

export function OlcrtcWbPanel({ onRoom, onInvalidate }: Props) {
  const { t } = useTranslation();
  const [messageApi, ctx] = message.useMessage();
  const [status, setStatus] = useState<WbStatus | null>(null);
  const [cookies, setCookies] = useState('');
  const [busy, setBusy] = useState(false);
  const [target, setTarget] = useState<number | null>(null);
  const [lastRoom, setLastRoom] = useState<{ id: string; link: string; inbound: number } | null>(
    null,
  );

  const inbounds = useMemo(() => status?.inbounds ?? [], [status]);
  const eligible = useMemo(
    () => inbounds.filter((ib) => ib.provider === 'wbstream' && !ib.remote),
    [inbounds],
  );

  const applyStatus = (st: WbStatus | null | undefined) => {
    if (st) setStatus(st);
  };

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const res = await tunnelsApi.wbStatus();
      if (!cancelled && res.success) applyStatus(res.obj);
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  // Default target: the first local WB Stream inbound, else the form below.
  useEffect(() => {
    if (target !== null || !status) return;
    setTarget(eligible.length > 0 ? eligible[0].id : FORM_TARGET);
  }, [status, eligible, target]);

  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    try {
      await fn();
    } finally {
      setBusy(false);
    }
  };

  const onSave = () =>
    run(async () => {
      const res = await tunnelsApi.wbSaveCookies(cookies.trim());
      if (res.success) {
        applyStatus(res.obj);
        setCookies('');
        messageApi.success(t('pages.tunnels.olcrtc.wb.savedCookies'));
      } else {
        messageApi.error(res.msg || t('pages.tunnels.olcrtc.wb.error'));
      }
    });

  const onClear = () =>
    run(async () => {
      const res = await tunnelsApi.wbClearCookies();
      if (res.success) {
        applyStatus(res.obj);
        setCookies('');
        messageApi.success(t('pages.tunnels.olcrtc.wb.clearedCookies'));
      } else {
        messageApi.error(res.msg || t('pages.tunnels.olcrtc.wb.error'));
      }
    });

  const onCreate = () =>
    run(async () => {
      const inboundId = target && target > 0 ? target : 0;
      const res = await tunnelsApi.wbCreate({ apply: inboundId > 0, inboundId });
      if (!res.success || !res.obj) {
        messageApi.error(res.msg || t('pages.tunnels.olcrtc.wb.error'));
        const st = await tunnelsApi.wbStatus();
        if (st.success) applyStatus(st.obj);
        return;
      }
      applyStatus(res.obj);
      const roomId = res.obj.room_id;
      setLastRoom({ id: roomId, link: res.obj.join_link, inbound: res.obj.applied_inbound_id });
      if (res.obj.applied_inbound_id > 0) {
        messageApi.success(
          t('pages.tunnels.olcrtc.wb.createdApplied', { id: res.obj.applied_inbound_id }),
        );
      } else {
        if (onRoom) onRoom(roomId);
        messageApi.success(t('pages.tunnels.olcrtc.wb.createdForm'));
      }
      onInvalidate?.();
    });

  const ok = Boolean(status?.cookies_ok);
  const expired = Boolean(status?.cookies_expired);
  const present = Boolean(status?.cookies_present);
  const names = status?.cookie_names ?? [];
  const rooms = (status?.rooms ?? []).slice(0, 5);

  const targetOptions = [
    {
      value: FORM_TARGET,
      label: onRoom
        ? t('pages.tunnels.olcrtc.wb.targetForm')
        : t('pages.tunnels.olcrtc.wb.targetNone'),
    },
    ...inbounds.map((ib) => {
      const bad = ib.provider !== 'wbstream' || ib.remote;
      const why = ib.remote
        ? t('pages.tunnels.olcrtc.wb.targetRemote')
        : ib.provider !== 'wbstream'
          ? t('pages.tunnels.olcrtc.wb.targetWrongProvider')
          : ib.room_id
            ? t('pages.tunnels.olcrtc.wb.targetReplace')
            : t('pages.tunnels.olcrtc.wb.targetEmpty');
      return {
        value: ib.id,
        disabled: bad,
        label: `#${ib.id} ${ib.remark || 'olcRTC'} — ${why}`,
      };
    }),
  ];

  return (
    <Card
      size="small"
      type="inner"
      title={t('pages.tunnels.olcrtc.wb.title')}
      style={{ marginBottom: 16 }}
      extra={
        <Tag color={ok ? 'green' : expired ? 'red' : present ? 'orange' : 'default'}>
          {ok
            ? t('pages.tunnels.olcrtc.wb.statusOk')
            : expired
              ? t('pages.tunnels.olcrtc.wb.statusExpired')
              : t('pages.tunnels.olcrtc.wb.statusMissing')}
        </Tag>
      }
    >
      {ctx}
      <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
        {t('pages.tunnels.olcrtc.wb.hint')}
      </Typography.Paragraph>

      {present && (
        <Typography.Paragraph style={{ fontSize: 12, marginBottom: 8 }}>
          <Typography.Text type="secondary">{t('pages.tunnels.olcrtc.wb.stored')} </Typography.Text>
          {names.map((n) => (
            <Tag key={n} color={n === 'wbx-refresh' ? 'green' : undefined}>
              {n}
            </Tag>
          ))}
          {status?.has_token && (
            <Tag color="blue">
              {status.token_exp
                ? t('pages.tunnels.olcrtc.wb.tokenUntil', { time: fmtTime(status.token_exp) })
                : t('pages.tunnels.olcrtc.wb.token')}
            </Tag>
          )}
          <br />
          <Typography.Text type={expired ? 'danger' : 'secondary'}>
            {expired
              ? t('pages.tunnels.olcrtc.wb.expiredHint')
              : status?.has_refresh
                ? t('pages.tunnels.olcrtc.wb.refreshHint')
                : t('pages.tunnels.olcrtc.wb.tokenOnlyHint')}
          </Typography.Text>
        </Typography.Paragraph>
      )}
      {status?.last_error ? (
        <Typography.Paragraph type="danger" style={{ fontSize: 12 }}>
          {t('pages.tunnels.olcrtc.wb.lastError')}: {status.last_error}
        </Typography.Paragraph>
      ) : null}

      <Typography.Text type="secondary">{t('pages.tunnels.olcrtc.wb.cookies')}</Typography.Text>
      <Input.TextArea
        rows={3}
        value={cookies}
        onChange={(e) => setCookies(e.target.value)}
        placeholder={t('pages.tunnels.olcrtc.wb.cookiesPlaceholder')}
        style={{ marginTop: 4, marginBottom: 8, fontFamily: 'monospace', fontSize: 12 }}
      />
      <Space wrap style={{ marginBottom: 12 }}>
        <Button disabled={busy || !cookies.trim()} loading={busy} onClick={() => void onSave()}>
          {t('pages.tunnels.olcrtc.wb.saveCookies')}
        </Button>
        <Button disabled={busy || !present} onClick={() => void onClear()}>
          {t('pages.tunnels.olcrtc.wb.clearCookies')}
        </Button>
      </Space>

      <div style={{ marginBottom: 8 }}>
        <Typography.Text type="secondary">{t('pages.tunnels.olcrtc.wb.target')}</Typography.Text>
        <Select
          style={{ width: '100%', marginTop: 4 }}
          value={target ?? FORM_TARGET}
          onChange={(v: number) => setTarget(v)}
          options={targetOptions}
        />
      </div>
      <Button
        type="primary"
        disabled={busy || !present}
        loading={busy}
        onClick={() => void onCreate()}
      >
        {t('pages.tunnels.olcrtc.wb.create')}
      </Button>

      {lastRoom && (
        <Alert
          type="success"
          showIcon
          style={{ marginTop: 12 }}
          message={
            lastRoom.inbound > 0
              ? t('pages.tunnels.olcrtc.wb.resultApplied', { id: lastRoom.inbound })
              : t('pages.tunnels.olcrtc.wb.resultForm')
          }
          description={
            <>
              <Typography.Paragraph copyable={{ text: lastRoom.id }} style={{ marginBottom: 4 }}>
                <Typography.Text code>{lastRoom.id}</Typography.Text>
              </Typography.Paragraph>
              <Typography.Link href={lastRoom.link} target="_blank" rel="noreferrer">
                {lastRoom.link}
              </Typography.Link>
            </>
          }
        />
      )}

      {rooms.length > 0 && (
        <div style={{ marginTop: 12 }}>
          <Typography.Text type="secondary">{t('pages.tunnels.olcrtc.wb.history')}</Typography.Text>
          {rooms.map((r) => (
            <div key={`${r.room_id}-${r.created_at}`} style={{ fontSize: 12 }}>
              <Typography.Text code copyable={{ text: r.room_id }}>
                {r.room_id}
              </Typography.Text>{' '}
              <Typography.Text type="secondary">
                {fmtTime(r.created_at)}
                {r.inbound_id ? ` → #${r.inbound_id}` : ''}
              </Typography.Text>
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}
