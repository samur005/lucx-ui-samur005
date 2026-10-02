import { useTranslation } from 'react-i18next';
import { AppstoreOutlined } from '@ant-design/icons';
import { Button, Dropdown, type MenuProps } from 'antd';

import { INBOUND_CREATE_TEMPLATES } from '@/lib/xray/inbound-templates';

interface InboundTemplatesButtonProps {
  onSelect: (templateId: string) => void;
  disabled?: boolean;
}

/**
 * Corner control for the inbound create/edit modal: opens a dropdown of
 * VLESS transport+security presets that auto-fill the form.
 */
export default function InboundTemplatesButton({
  onSelect,
  disabled,
}: InboundTemplatesButtonProps) {
  const { t } = useTranslation();

  const items: MenuProps['items'] = INBOUND_CREATE_TEMPLATES.map((tmpl) => ({
    key: tmpl.id,
    label: t(tmpl.labelKey),
  }));

  return (
    <Dropdown
      trigger={['click']}
      disabled={disabled}
      menu={{
        items,
        onClick: ({ key }) => onSelect(String(key)),
      }}
    >
      <Button size="small" icon={<AppstoreOutlined />} disabled={disabled}>
        {t('pages.inbounds.form.templates.button')}
      </Button>
    </Dropdown>
  );
}
