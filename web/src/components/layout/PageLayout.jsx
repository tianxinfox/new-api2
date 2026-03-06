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

import HeaderBar from './headerbar';
import { Layout } from '@douyinfe/semi-ui';
import SiderBar from './SiderBar';
import App from '../../App';
import { ToastContainer } from 'react-toastify';
import React, { useContext, useEffect, useState } from 'react';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { useSidebarCollapsed } from '../../hooks/common/useSidebarCollapsed';
import { useTranslation } from 'react-i18next';
import {
  API,
  getLogo,
  getSystemName,
  showError,
  setStatusData,
} from '../../helpers';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';
import { useLocation } from 'react-router-dom';
const { Sider, Content, Header } = Layout;

const ensureMetaTag = ({ name, property, content }) => {
  if (typeof document === 'undefined') return;
  const selector = name
    ? `meta[name="${name}"]`
    : `meta[property="${property}"]`;
  let meta = document.head.querySelector(selector);
  if (!meta) {
    meta = document.createElement('meta');
    if (name) {
      meta.setAttribute('name', name);
    } else if (property) {
      meta.setAttribute('property', property);
    }
    document.head.appendChild(meta);
  }
  meta.setAttribute('content', content);
};

const ensureCanonicalLink = (href) => {
  if (typeof document === 'undefined') return;
  let link = document.head.querySelector('link[rel="canonical"]');
  if (!link) {
    link = document.createElement('link');
    link.setAttribute('rel', 'canonical');
    document.head.appendChild(link);
  }
  link.setAttribute('href', href);
};

const getSeoBaseUrl = (status) => {
  if (status?.server_address) {
    try {
      const url = new URL(status.server_address);
      return url.origin;
    } catch {
      // ignore invalid configured URL and fallback to window origin
    }
  }
  if (typeof window !== 'undefined') {
    return window.location.origin;
  }
  return '';
};

const isZhLanguage = (language) => {
  if (!language) return true;
  return language.toLowerCase().startsWith('zh');
};

const toOgLocale = (language) => {
  if (!language) return 'zh_CN';
  const normalized = language.toLowerCase();
  if (normalized === 'zh-cn' || normalized === 'zh') return 'zh_CN';
  if (normalized === 'zh-tw') return 'zh_TW';
  if (normalized === 'en' || normalized.startsWith('en-')) return 'en_US';
  if (normalized === 'ja' || normalized.startsWith('ja-')) return 'ja_JP';
  if (normalized === 'fr' || normalized.startsWith('fr-')) return 'fr_FR';
  if (normalized === 'ru' || normalized.startsWith('ru-')) return 'ru_RU';
  if (normalized === 'vi' || normalized.startsWith('vi-')) return 'vi_VN';
  return 'zh_CN';
};

const buildSeoMeta = ({ pathname, systemName, baseUrl, language }) => {
  const normalizedPath = pathname || '/';
  const isZh = isZhLanguage(language);
  const noIndexPrefixes = ['/console', '/api/', '/v1/', '/mj/', '/pg/', '/oauth'];
  const noIndexPrefixExactPaths = ['/api', '/v1', '/mj', '/pg'];
  const noIndexExactPaths = ['/login', '/register', '/reset', '/user/reset'];
  const shouldNoIndex =
    noIndexPrefixes.some((prefix) => normalizedPath.startsWith(prefix)) ||
    noIndexPrefixExactPaths.includes(normalizedPath) ||
    normalizedPath === '/setup' ||
    normalizedPath === '/chat2link' ||
    noIndexExactPaths.includes(normalizedPath);

  const pageMetaMap = {
    '/': {
      title: isZh ? `${systemName} - 大模型接口网关` : `${systemName} - AI API Gateway`,
      description: isZh
        ? '统一的 AI API 网关，聚合 OpenAI、Claude、Gemini、Azure、Bedrock 等渠道，支持密钥管理、计费与监控。'
        : 'Unified AI API gateway for OpenAI, Claude, Gemini, Azure, Bedrock and more with key management, billing and monitoring.',
    },
    '/pricing': {
      title: isZh ? `价格 - ${systemName}` : `Pricing - ${systemName}`,
      description: isZh
        ? '集中对比模型价格与 Token 成本，帮助你为 AI 业务选择更合适的渠道。'
        : 'Compare model pricing and token costs in one place to choose the best channel for your AI workloads.',
    },
    '/about': {
      title: isZh ? `关于 - ${systemName}` : `About - ${systemName}`,
      description: isZh
        ? '了解平台能力、项目背景与部署方式。'
        : 'Learn about the platform, project background, and deployment model of this AI API gateway.',
    },
    '/privacy-policy': {
      title: isZh ? `隐私政策 - ${systemName}` : `Privacy Policy - ${systemName}`,
      description: isZh
        ? '查看本平台如何收集、使用与保护个人数据。'
        : 'Review how personal data is collected, used, and protected on this platform.',
    },
    '/user-agreement': {
      title: isZh ? `用户协议 - ${systemName}` : `User Agreement - ${systemName}`,
      description: isZh
        ? '使用平台前请阅读服务条款与使用条件。'
        : 'Read the service terms and usage conditions before using this platform.',
    },
  };

  const defaultMeta = {
    title: systemName,
    description: isZh
      ? '统一的 AI API 网关，支持多供应商中继、额度控制、计费与运营管理。'
      : 'Unified AI API gateway with multi-provider relay, quota control, billing and operational dashboard.',
  };
  const selected = pageMetaMap[normalizedPath] || defaultMeta;
  const canonicalUrl = `${baseUrl}${normalizedPath === '/' ? '/' : normalizedPath}`;

  return {
    title: selected.title,
    description: selected.description,
    canonicalUrl,
    robots: shouldNoIndex ? 'noindex, nofollow' : 'index, follow',
  };
};

const PageLayout = () => {
  const [, userDispatch] = useContext(UserContext);
  const [statusState, statusDispatch] = useContext(StatusContext);
  const isMobile = useIsMobile();
  const [collapsed, , setCollapsed] = useSidebarCollapsed();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const { i18n } = useTranslation();
  const location = useLocation();

  const shouldInnerPadding =
    location.pathname.includes('/console') &&
    !location.pathname.startsWith('/console/chat') &&
    location.pathname !== '/console/playground';

  const isConsoleRoute = location.pathname.startsWith('/console');
  const showSider = isConsoleRoute && (!isMobile || drawerOpen);

  useEffect(() => {
    if (isMobile && drawerOpen && collapsed) {
      setCollapsed(false);
    }
  }, [isMobile, drawerOpen, collapsed, setCollapsed]);

  const loadUser = () => {
    let user = localStorage.getItem('user');
    if (user) {
      let data = JSON.parse(user);
      userDispatch({ type: 'login', payload: data });
    }
  };

  const loadStatus = async () => {
    try {
      const res = await API.get('/api/status');
      const { success, data } = res.data;
      if (success) {
        statusDispatch({ type: 'set', payload: data });
        setStatusData(data);
      } else {
        showError('Unable to connect to server');
      }
    } catch (error) {
      showError('Failed to load status');
    }
  };

  useEffect(() => {
    loadUser();
    loadStatus().catch(console.error);
    let logo = getLogo();
    if (logo) {
      let linkElement = document.querySelector("link[rel~='icon']");
      if (linkElement) {
        linkElement.href = logo;
      }
    }
    const savedLang = localStorage.getItem('i18nextLng');
    if (savedLang) {
      i18n.changeLanguage(savedLang);
    }
  }, [i18n]);

  useEffect(() => {
    const systemName = statusState?.status?.system_name || getSystemName();
    const logo = statusState?.status?.logo || getLogo();
    let status = statusState?.status;
    if (!status) {
      try {
        status = JSON.parse(localStorage.getItem('status') || '{}');
      } catch {
        status = {};
      }
    }
    const baseUrl = getSeoBaseUrl(status);
    const seoMeta = buildSeoMeta({
      pathname: location.pathname,
      systemName,
      baseUrl,
      language: i18n.language,
    });

    document.title = seoMeta.title;
    document.documentElement.lang = i18n.language || 'zh-CN';
    ensureMetaTag({ name: 'description', content: seoMeta.description });
    ensureMetaTag({ name: 'robots', content: seoMeta.robots });
    ensureCanonicalLink(seoMeta.canonicalUrl);
    ensureMetaTag({ property: 'og:type', content: 'website' });
    ensureMetaTag({ property: 'og:locale', content: toOgLocale(i18n.language) });
    ensureMetaTag({ property: 'og:title', content: seoMeta.title });
    ensureMetaTag({ property: 'og:description', content: seoMeta.description });
    ensureMetaTag({ property: 'og:url', content: seoMeta.canonicalUrl });
    ensureMetaTag({ property: 'og:site_name', content: systemName });
    ensureMetaTag({ name: 'twitter:card', content: 'summary' });
    ensureMetaTag({ name: 'twitter:title', content: seoMeta.title });
    ensureMetaTag({
      name: 'twitter:description',
      content: seoMeta.description,
    });
    if (logo && baseUrl) {
      const imageUrl = logo.startsWith('http') ? logo : `${baseUrl}${logo}`;
      ensureMetaTag({ property: 'og:image', content: imageUrl });
      ensureMetaTag({ name: 'twitter:image', content: imageUrl });
    }
  }, [
    location.pathname,
    i18n.language,
    statusState?.status?.system_name,
    statusState?.status?.logo,
    statusState?.status?.server_address,
  ]);

  return (
    <Layout
      className='app-layout'
      style={{
        display: 'flex',
        flexDirection: 'column',
        overflow: isMobile ? 'visible' : 'hidden',
      }}
    >
      <Header
        style={{
          padding: 0,
          height: 'auto',
          lineHeight: 'normal',
          position: 'fixed',
          width: '100%',
          top: 0,
          zIndex: 100,
        }}
      >
        <HeaderBar
          onMobileMenuToggle={() => setDrawerOpen((prev) => !prev)}
          drawerOpen={drawerOpen}
        />
      </Header>
      <Layout
        style={{
          overflow: isMobile ? 'visible' : 'auto',
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        {showSider && (
          <Sider
            className='app-sider'
            style={{
              position: 'fixed',
              left: 0,
              top: '64px',
              zIndex: 99,
              border: 'none',
              paddingRight: '0',
              width: 'var(--sidebar-current-width)',
            }}
          >
            <SiderBar
              onNavigate={() => {
                if (isMobile) setDrawerOpen(false);
              }}
            />
          </Sider>
        )}
        <Layout
          style={{
            marginLeft: isMobile
              ? '0'
              : showSider
                ? 'var(--sidebar-current-width)'
                : '0',
            flex: '1 1 auto',
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          <Content
            style={{
              flex: '1 0 auto',
              overflowY: isMobile ? 'visible' : 'hidden',
              WebkitOverflowScrolling: 'touch',
              padding: shouldInnerPadding ? (isMobile ? '5px' : '24px') : '0',
              position: 'relative',
            }}
          >
            <App />
          </Content>
        </Layout>
      </Layout>
      <ToastContainer />
    </Layout>
  );
};

export default PageLayout;
