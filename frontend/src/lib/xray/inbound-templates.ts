/*
 * Preset profiles for the inbound create/edit form "Templates" dropdown.
 * Each entry maps to protocol=VLESS plus a transport/network + security
 * combination. Application logic lives in InboundFormModal so it can reuse
 * onNetworkChange / onSecurityChange (Reality keypair, TLS cert seed, KCP
 * finalmask, etc.).
 */

export type InboundTemplateSecurity = 'reality' | 'tls' | 'none';
export type InboundTemplateNetwork = 'xhttp' | 'grpc' | 'ws' | 'httpupgrade' | 'kcp';

export interface InboundCreateTemplate {
  id: string;
  /** i18n key under pages.inbounds.form.templates.* */
  labelKey: string;
  network: InboundTemplateNetwork;
  security: InboundTemplateSecurity;
}

export const INBOUND_CREATE_TEMPLATES: readonly InboundCreateTemplate[] = [
  {
    id: 'vless-xhttp-reality',
    labelKey: 'pages.inbounds.form.templates.vlessXhttpReality',
    network: 'xhttp',
    security: 'reality',
  },
  {
    id: 'vless-grpc-reality',
    labelKey: 'pages.inbounds.form.templates.vlessGrpcReality',
    network: 'grpc',
    security: 'reality',
  },
  {
    id: 'vless-grpc-tls',
    labelKey: 'pages.inbounds.form.templates.vlessGrpcTls',
    network: 'grpc',
    security: 'tls',
  },
  {
    id: 'vless-ws-tls',
    labelKey: 'pages.inbounds.form.templates.vlessWsTls',
    network: 'ws',
    security: 'tls',
  },
  {
    id: 'vless-httpupgrade-tls',
    labelKey: 'pages.inbounds.form.templates.vlessHttpupgradeTls',
    network: 'httpupgrade',
    security: 'tls',
  },
  {
    id: 'vless-kcp',
    labelKey: 'pages.inbounds.form.templates.vlessKcp',
    network: 'kcp',
    security: 'none',
  },
] as const;

export function findInboundCreateTemplate(id: string): InboundCreateTemplate | undefined {
  return INBOUND_CREATE_TEMPLATES.find((t) => t.id === id);
}
