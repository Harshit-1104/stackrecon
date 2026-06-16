import type { NextConfig } from "next";
import fs from "fs";
import path from "path";

// Load .env from root since web/ is a subdirectory
const envPath = path.resolve(process.cwd(), '../.env');
if (fs.existsSync(envPath)) {
  const envConfig = fs.readFileSync(envPath, 'utf-8');
  envConfig.split('\n').forEach(line => {
    const match = line.match(/^([^=]+)=(.*)$/);
    if (match) {
      const key = match[1].trim();
      const value = match[2].trim();
      if (!process.env[key]) {
        process.env[key] = value;
      }
    }
  });
}

const nextConfig: NextConfig = {
  /* config options here */
  output: 'standalone',
  env: {
    NEXT_PUBLIC_LOGO_DEV_TOKEN: process.env.NEXT_PUBLIC_LOGO_DEV_TOKEN || '',
    ACTIVE_THRESHOLD_DAYS: process.env.ACTIVE_THRESHOLD_DAYS || '',
  }
};

export default nextConfig;
