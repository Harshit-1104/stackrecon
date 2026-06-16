'use client';

import React, { useState } from 'react';
import CompanyLogo from './CompanyLogo';
import { CompanyModal } from './CompanyModal';

interface TopSkill {
  name: string;
  is_rare: boolean;
}

interface CompanyCardProps {
  company: {
    company_id: number;
    name: string;
    domain: string;
    rail: number;
    company_score: number;
    jd_score: number;
    qualified_jd_count: number;
    top_matched_skills: TopSkill[];
  };
  maxQualifiedInRail?: number;
}

export function CompanyCard({ company, maxQualifiedInRail }: CompanyCardProps) {
  const [isModalOpen, setIsModalOpen] = useState(false);

  const scorePct = Math.round((company.company_score || 0) * 100);

  let barColor = 'bg-success';
  if (company.rail === 2) barColor = 'bg-indigo-500';
  if (company.rail === 3) barColor = 'bg-zinc-500';

  const barWidth = (company.rail !== 3 && maxQualifiedInRail && maxQualifiedInRail > 0)
    ? Math.min((company.qualified_jd_count / maxQualifiedInRail) * 100, 100)
    : Math.min(scorePct, 100);

  return (
    <>
      <div 
        onClick={() => setIsModalOpen(true)}
        className="w-full bg-bg-surface border border-border rounded-xl p-4 hover:border-accent-mid transition-all cursor-pointer flex flex-col shrink-0 group"
      >
        <div className="flex items-center gap-3">
          <CompanyLogo domain={company.domain} name={company.name} size={32} />
          <h3 className="font-medium text-[14px] text-text-primary truncate flex-1" title={company.name}>{company.name}</h3>
          <span className="text-text-tertiary group-hover:text-accent transition-colors">→</span>
        </div>
        
        {company.top_matched_skills && company.top_matched_skills.length > 0 && (
          <div className="mt-3 text-[12px] line-clamp-2 leading-relaxed text-text-secondary border-t border-border pt-3">
            {company.top_matched_skills?.map((skill, idx) => (
              <React.Fragment key={skill.name}>
                {idx > 0 && <span className="text-text-tertiary"> · </span>}
                <span className={skill.is_rare ? "text-text-primary font-medium" : "text-text-secondary"}>
                  {skill.name} {skill.is_rare && <span className="text-rare ml-0.5">★</span>}
                </span>
              </React.Fragment>
            ))}
          </div>
        )}

        <div className="mt-3">
          <div className={`flex items-end mb-1.5 text-[12px] ${company.rail !== 3 ? 'justify-end' : 'justify-between'}`}>
            {company.rail === 3 && (
              <span className="font-medium text-text-primary">{scorePct}% Match</span>
            )}
            <span className={company.rail === 3 && company.qualified_jd_count === 0 ? "text-accent font-medium" : "text-text-tertiary"}>
              {company.rail === 2 
                ? (company.qualified_jd_count === 0 ? 'Reach out' : `${company.qualified_jd_count} qualified roles`)
                : `${company.qualified_jd_count} qualified roles`
              }
            </span>
          </div>
          <div className="w-full bg-bg-subtle rounded-full h-1 overflow-hidden">
            <div 
              className={`h-1 rounded-full ${barColor.replace('bg-success', 'bg-match')}`} 
              style={{ width: `${barWidth}%` }}
            />
          </div>
        </div>
      </div>

      {isModalOpen && (
        <CompanyModal company={company} onClose={() => setIsModalOpen(false)} />
      )}
    </>
  );
}
