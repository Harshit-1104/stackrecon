'use client';

import { useState, useEffect, useRef } from 'react';
import { useRouter } from 'next/navigation';
import { useSkillStore } from '@/lib/store';
import { searchCompanies, searchSkills } from '@/lib/api';
import { Search, Building2, Plus, X, Command } from 'lucide-react';
import CompanyLogo from '@/components/CompanyLogo';
import { SkillChip } from '@/components/SkillChip';

export default function Home() {
  const router = useRouter();
  const { userSkills, addSkill, removeSkill } = useSkillStore();
  
  const [companyQuery, setCompanyQuery] = useState('');
  const [companyResults, setCompanyResults] = useState<any[]>([]);
  const [topCompanies, setTopCompanies] = useState<any[]>([]);
  const [isSearchingCompany, setIsSearchingCompany] = useState(false);
  const companyInputRef = useRef<HTMLInputElement>(null);

  const [skillQuery, setSkillQuery] = useState('');
  const [skillResults, setSkillResults] = useState<any[]>([]);
  const [isAddingSkill, setIsAddingSkill] = useState(false);
  const skillInputRef = useRef<HTMLInputElement>(null);

  // Focus shortcut
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        companyInputRef.current?.focus();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  // Fetch top companies on mount
  useEffect(() => {
    searchCompanies('').then(res => setTopCompanies(res || [])).catch(console.error);
  }, []);

  // Company Search Effect
  useEffect(() => {
    if (companyQuery.length > 1) {
      setIsSearchingCompany(true);
      const delay = setTimeout(async () => {
        try {
          const res = await searchCompanies(companyQuery);
          setCompanyResults(res || []);
        } catch (e) {
          console.error(e);
        } finally {
          setIsSearchingCompany(false);
        }
      }, 300);
      return () => clearTimeout(delay);
    } else {
      setCompanyResults([]);
      setIsSearchingCompany(false);
    }
  }, [companyQuery]);

  // Skill Search Effect
  useEffect(() => {
    if (skillQuery.length > 1) {
      const delay = setTimeout(async () => {
        try {
          const res = await searchSkills(skillQuery);
          setSkillResults(res || []);
        } catch (e) {
          console.error(e);
        }
      }, 300);
      return () => clearTimeout(delay);
    } else {
      setSkillResults([]);
    }
  }, [skillQuery]);

  return (
    <div className="flex-1 flex flex-col items-center pt-24 md:pt-32 p-6 text-center z-10 relative">
      <div className="max-w-3xl w-full">
        <h1 className="text-5xl md:text-7xl font-bold tracking-tight mb-6 text-text-primary">
          Company-First Stack Search
        </h1>
        <p className="text-xl text-text-secondary mb-12 max-w-2xl mx-auto">
          Find out what tech stack top companies actually use, backed by data from thousands of job postings.
        </p>

        {/* Company Search Bar */}
        <div className="relative w-full max-w-2xl mx-auto mb-16">
          <div className={`flex items-center bg-bg-surface border ${companyQuery ? 'border-accent-border ring-2 ring-accent-light' : 'border-border'} rounded-2xl p-4 shadow-sm transition-all duration-300 focus-within:border-accent focus-within:ring-2 focus-within:ring-accent-light`}>
            <Search className="text-text-tertiary mr-3 shrink-0" size={24} />
            <input
              ref={companyInputRef}
              type="text"
              className="bg-transparent text-xl text-text-primary w-full focus:outline-none placeholder:text-text-tertiary"
              placeholder="Search a company (e.g. Stripe, Netflix)..."
              value={companyQuery}
              onChange={e => setCompanyQuery(e.target.value)}
            />
            {!companyQuery && (
              <div className="hidden sm:flex items-center gap-1 text-xs text-text-tertiary font-medium px-2 py-1 bg-bg-subtle rounded-md border border-border">
                <Command size={12} /> K
              </div>
            )}
            {isSearchingCompany && (
              <div className="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin shrink-0"></div>
            )}
          </div>

          {/* Company Autocomplete Dropdown */}
          {companyResults.length > 0 && (
            <div className="absolute left-0 right-0 top-full mt-2 bg-bg-surface border border-border rounded-xl shadow-md z-30 max-h-96 overflow-y-auto overflow-x-hidden p-2 text-left">
              {companyResults.map(company => (
                <button
                  key={company.id}
                  onClick={() => router.push(`/companies/${company.id}`)}
                  className="w-full flex flex-col sm:flex-row sm:items-center gap-4 p-3 hover:bg-bg-subtle rounded-lg transition-colors group"
                >
                  <CompanyLogo domain={company.website} name={company.name} size={48} />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-baseline justify-between mb-1">
                      <span className="font-semibold text-text-primary text-lg truncate pr-2">{company.name}</span>
                      <span className="text-xs text-text-secondary whitespace-nowrap bg-bg-subtle px-2 py-0.5 rounded-full border border-border">
                        {company.total_jd_count.toLocaleString()} jobs
                      </span>
                    </div>
                    {company.top_skills && company.top_skills.length > 0 && (
                      <div className="flex flex-wrap gap-1.5">
                        {company.top_skills.map((s: any) => (
                          <span key={s.name} className="text-xs text-text-secondary bg-bg-subtle border border-border-subtle px-2 py-0.5 rounded-md truncate max-w-[120px]">
                            {s.name}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Top Companies Grid */}
        {!companyQuery && topCompanies.length > 0 && (
          <div className="max-w-4xl mx-auto mt-8 text-left animate-in fade-in slide-in-from-bottom-4 duration-500">
            <h2 className="text-[13px] font-semibold text-text-secondary uppercase tracking-[0.08em] mb-4 px-2">Featured Companies</h2>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
              {topCompanies.map(company => (
                <button
                  key={company.id}
                  onClick={() => router.push(`/companies/${company.id}`)}
                  className="flex items-center gap-3 p-3 bg-bg-surface border border-border rounded-xl hover:border-accent hover:shadow-sm transition-all text-left group"
                >
                  <CompanyLogo domain={company.website} name={company.name} size={36} />
                  <div className="min-w-0">
                    <h3 className="font-semibold text-text-primary text-[14px] truncate group-hover:text-accent transition-colors">{company.name}</h3>
                    <p className="text-[11px] text-text-secondary mt-0.5">{company.total_jd_count.toLocaleString()} active jobs</p>
                  </div>
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Optional Skills Section */}
        
        {/* <div className="max-w-2xl mx-auto bg-surface/50 border border-border/50 rounded-2xl p-6 md:p-8 backdrop-blur-sm text-left">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
            <div>
              <h2 className="text-lg font-medium text-foreground flex items-center gap-2">
                Personalize your matches <span className="text-xs bg-accent/20 text-accent px-2 py-0.5 rounded-full border border-accent/30 font-medium">Optional</span>
              </h2>
              <p className="text-sm text-muted mt-1">Add your skills to see how well you fit with each company.</p>
            </div>
          </div>

          <div className="flex flex-wrap gap-3">
            {userSkills.map((s: any) => (
              <SkillChip 
                key={s.skill_id} 
                skill={s} 
                onRemove={removeSkill}
              />
            ))}
            
            <div className="relative">
              {!isAddingSkill ? (
                <button 
                  onClick={() => setIsAddingSkill(true)}
                  className="inline-flex items-center gap-1.5 px-4 py-2 rounded-full text-sm border border-dashed border-border text-muted hover:text-foreground hover:bg-white/5 hover:border-accent/50 transition-all shadow-sm"
                >
                  <Plus size={16} /> Add skill
                </button>
              ) : (
                <div className="flex items-center bg-background border border-accent rounded-full pl-4 pr-1 py-1 w-[240px] shadow-lg ring-2 ring-accent/20">
                  <Search size={14} className="text-muted mr-2 shrink-0" />
                  <input
                    ref={skillInputRef}
                    autoFocus
                    type="text"
                    className="bg-transparent text-sm w-full focus:outline-none"
                    placeholder="Type a skill..."
                    value={skillQuery}
                    onChange={e => setSkillQuery(e.target.value)}
                    onBlur={() => setTimeout(() => setIsAddingSkill(false), 200)}
                  />
                  <button onClick={() => setIsAddingSkill(false)} className="p-1.5 hover:bg-white/10 rounded-full text-muted hover:text-foreground">
                    <X size={14} />
                  </button>
                </div>
              )} */}

              {/* Skill Autocomplete Dropdown */}
              {/* {isAddingSkill && skillResults.length > 0 && (
                <div className="absolute left-0 top-full mt-2 bg-surface border border-border rounded-xl shadow-2xl z-30 max-h-60 overflow-y-auto w-[240px]">
                  {skillResults.map(res => {
                    const isSelected = userSkills.some(s => s.skill_id === res.id);
                    return (
                      <button
                        key={res.id}
                        disabled={isSelected}
                        className={`w-full text-left px-4 py-3 hover:bg-accent hover:text-white transition-colors border-b border-border/50 last:border-0 flex flex-col gap-0.5 ${isSelected ? 'opacity-50 cursor-not-allowed' : ''}`}
                        onMouseDown={(e) => {
                          e.preventDefault();
                          if (!isSelected) {
                            addSkill({
                              skill_id: res.id,
                              skill_name: res.canonical_name,
                              canonical_name: res.canonical_name,
                              category: res.category
                            });
                            setSkillQuery('');
                            setSkillResults([]);
                            setIsAddingSkill(false);
                          }
                        }}
                      >
                        <span className="text-sm font-medium">{res.canonical_name}</span>
                        <span className="text-xs opacity-70">{res.category || 'Other'}</span>
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          </div> */}
        {/* </div> */}
      </div>
    </div>
  );
}
