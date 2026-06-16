'use client';

import { useEffect, useState } from 'react';
import { X, ExternalLink, Check } from 'lucide-react';
import CompanyLogo from './CompanyLogo';
import { useSkillStore } from '@/lib/store';
import { getCompanyPostings } from '@/lib/api';

interface Posting {
  id: string;
  title: string;
  apply_url: string;
  matched_skills: string[];
  posted_at: string;
  min_years_required?: number;
  total_skills_in_job?: number;
}

export function CompanyModal({ company, onClose }: { company: any, onClose: () => void }) {
  const { userSkills, total_years_experience, work_types, countries, cities } = useSkillStore();
  const [fetchedPostings, setFetchedPostings] = useState<Posting[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedSkills, setSelectedSkills] = useState<Set<string>>(new Set());
  const [availableSkills, setAvailableSkills] = useState<string[]>([]);

  useEffect(() => {
    const fetchPostings = async () => {
      try {
        setLoading(true);
        const skillIds = userSkills.filter(s => s.skill_id).map(s => s.skill_id).join(',');
        const res = await getCompanyPostings(company.company_id, skillIds, total_years_experience, work_types, countries, cities);
        const posts = res.postings || [];
        
        posts.sort((a: Posting, b: Posting) => {
          const ratioA = (a.matched_skills?.length || 0) / Math.max(a.total_skills_in_job || 1, 1);
          const ratioB = (b.matched_skills?.length || 0) / Math.max(b.total_skills_in_job || 1, 1);
          return ratioB - ratioA;
        });
        
        setFetchedPostings(posts);
        
        const skills = Array.from(new Set(posts.flatMap((p: Posting) => p.matched_skills || []))).sort();
        setAvailableSkills(skills as string[]);
        setSelectedSkills(new Set(skills as string[]));
      } catch (err) {
        console.error(err);
      } finally {
        setLoading(false);
      }
    };
    fetchPostings();
  }, [company.company_id, userSkills, total_years_experience, work_types, countries, cities]);

  const scorePct = Math.min(100, Math.round((company.company_score || 0) * 100));

  const filteredPostings = fetchedPostings.filter(p => {
    if (selectedSkills.size === 0) return false;
    return p.matched_skills?.some(s => selectedSkills.has(s));
  });

  const toggleSkill = (skill: string) => {
    const next = new Set(selectedSkills);
    if (next.has(skill)) {
      next.delete(skill);
    } else {
      next.add(skill);
    }
    setSelectedSkills(next);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div 
        className="absolute inset-0" 
        onClick={onClose}
      />
      <div className="relative bg-surface border border-border w-full max-w-2xl rounded-xl shadow-2xl flex flex-col max-h-[85vh]">
        
        {/* Header */}
        <div className="p-6 border-b border-border flex items-start justify-between shrink-0">
          <div className="flex items-center gap-4">
            <CompanyLogo domain={company.domain} name={company.name} size={48} />
            <div>
              <h2 className="text-xl font-bold">{company.name}</h2>
              <p className="text-sm text-muted mt-1">
                {company.qualified_jd_count} qualified roles · {scorePct}% stack match
              </p>
            </div>
          </div>
          <button onClick={onClose} className="p-2 text-muted hover:text-foreground transition-colors rounded-lg hover:bg-white/5">
            <X size={20} />
          </button>
        </div>

        {/* Content */}
        <div className="p-6 overflow-y-auto min-h-[30vh]">
          {availableSkills.length > 0 && !loading && (
            <div className="mb-6">
              <h3 className="text-sm font-medium text-muted mb-3">Filter by skill:</h3>
              <div className="flex flex-wrap gap-2">
                {availableSkills.map(skill => {
                  const isActive = selectedSkills.has(skill);
                  return (
                    <button
                      key={skill}
                      onClick={() => toggleSkill(skill)}
                      className={`flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm font-medium transition-colors border ${
                        isActive 
                          ? 'bg-accent/20 border-accent/30 text-accent' 
                          : 'bg-white/5 border-border text-muted hover:text-foreground hover:border-muted'
                      }`}
                    >
                      {skill} {isActive && <Check size={14} />}
                    </button>
                  );
                })}
              </div>
              <p className="text-sm text-muted mt-3">
                Showing {filteredPostings.length} of {fetchedPostings.length} roles
              </p>
            </div>
          )}

          {loading ? (
            <div className="animate-pulse space-y-4">
              {[1,2,3].map(i => (
                <div key={i} className="h-16 bg-white/5 rounded-lg" />
              ))}
            </div>
          ) : fetchedPostings.length > 0 ? (
            filteredPostings.length > 0 ? (
              <div className="space-y-4">
                {filteredPostings.map((p, idx) => (
                  <div key={p.id}>
                    <div className="flex justify-between items-start">
                      <div>
                        <div className="flex items-center gap-2 flex-wrap">
                          <h4 className="font-medium text-foreground">
                            {p.title}
                          </h4>
                          {p.min_years_required !== null && p.min_years_required !== undefined && (
                            <span className="text-xs px-2 py-0.5 rounded-full bg-white/5 text-muted border border-border">
                              {p.min_years_required}+ yrs
                            </span>
                          )}
                          <span className="text-xs px-2 py-0.5 rounded-full bg-white/5 text-muted border border-border">
                            {p.matched_skills?.length || 0}/{p.total_skills_in_job || 0} skills
                          </span>
                        </div>
                        <p className="text-sm text-muted mt-2">{p.matched_skills?.join(' · ')}</p>
                      </div>
                      {p.apply_url && (
                        <a 
                          href={p.apply_url} 
                          target="_blank" 
                          rel="noreferrer"
                          className="flex items-center gap-1 text-sm font-medium text-accent hover:text-accent/80 transition-colors shrink-0 ml-4"
                        >
                          Apply <ExternalLink size={14} />
                        </a>
                      )}
                    </div>
                    {idx < filteredPostings.length - 1 && <hr className="border-border mt-4" />}
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-center py-8">
                <p className="text-muted">No roles match the selected skills.</p>
              </div>
            )
          ) : (
            <div className="text-center py-8">
              {company.rail === 3 ? (
                <>
                  <p className="text-muted mb-4">No open roles currently.</p>
                  <a 
                    href={`https://www.linkedin.com/company/${company.name.toLowerCase().replace(/[^a-z0-9]+/g, '-')}`}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-1 text-sm font-medium text-accent hover:text-accent/80 transition-colors"
                  >
                    Connect on LinkedIn <ExternalLink size={14} />
                  </a>
                </>
              ) : (
                <>
                  <p className="text-muted mb-4">No open roles matching your experience level.</p>
                  <a 
                    href={`https://www.linkedin.com/company/${company.name.toLowerCase().replace(/[^a-z0-9]+/g, '-')}`}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-1 text-sm font-medium text-accent hover:text-accent/80 transition-colors"
                  >
                    Connect on LinkedIn <ExternalLink size={14} />
                  </a>
                </>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
