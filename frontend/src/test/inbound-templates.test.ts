import { describe, expect, it } from 'vitest';

import { INBOUND_CREATE_TEMPLATES, findInboundCreateTemplate } from '@/lib/xray/inbound-templates';
import { canEnableReality, canEnableTls } from '@/lib/xray/protocol-capabilities';

describe('inbound create templates', () => {
  it('exposes the six VLESS presets from the product brief', () => {
    expect(INBOUND_CREATE_TEMPLATES.map((t) => t.id)).toEqual([
      'vless-xhttp-reality',
      'vless-grpc-reality',
      'vless-grpc-tls',
      'vless-ws-tls',
      'vless-httpupgrade-tls',
      'vless-kcp',
    ]);
  });

  it('maps each template to a capability-legal network+security pair', () => {
    for (const tmpl of INBOUND_CREATE_TEMPLATES) {
      const slice = {
        protocol: 'vless',
        streamSettings: { network: tmpl.network, security: tmpl.security },
      };
      if (tmpl.security === 'reality') {
        expect(canEnableReality(slice), tmpl.id).toBe(true);
      } else if (tmpl.security === 'tls') {
        expect(canEnableTls(slice), tmpl.id).toBe(true);
      } else {
        expect(tmpl.network).toBe('kcp');
      }
    }
  });

  it('findInboundCreateTemplate resolves known ids', () => {
    expect(findInboundCreateTemplate('vless-ws-tls')?.network).toBe('ws');
    expect(findInboundCreateTemplate('missing')).toBeUndefined();
  });
});
