'use client';

import { useState } from 'react';

interface CompanyLogoProps {
  domain: string | null;
  name: string;
  size?: number;
}

export default function CompanyLogo({ domain, name, size = 40 }: CompanyLogoProps) {
  const [error, setError] = useState(false);
  const token = process.env.NEXT_PUBLIC_LOGO_DEV_TOKEN;

  const initials = name.substring(0, 2).toUpperCase();

  // Clean the domain for logo.dev
  let cleanDomain = domain || '';
  if (cleanDomain) {
    try {
      if (!cleanDomain.startsWith('http')) {
        cleanDomain = 'https://' + cleanDomain;
      }
      const url = new URL(cleanDomain);
      cleanDomain = url.hostname.replace(/^www\./, '');
    } catch (e) {
      cleanDomain = domain || '';
    }
  }

  const showImage = cleanDomain && !error && token;

  if (showImage) {
    return (
      <img
        src={`https://img.logo.dev/${cleanDomain}?token=${token}`}
        alt={`${name} logo`}
        style={{ width: size, height: size, objectFit: 'contain' }}
        className="rounded-lg shrink-0"
        onError={() => setError(true)}
      />
    );
  }

  return (
    <div
      style={{ width: size, height: size }}
      className="bg-accent text-white flex items-center justify-center rounded-lg font-bold shrink-0"
      title={name}
    >
      {initials}
    </div>
  );
}
