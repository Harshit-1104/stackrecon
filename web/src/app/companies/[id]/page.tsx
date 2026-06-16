'use client';

import { useEffect, useState, use } from 'react';
import { useRouter } from 'next/navigation';
import { getCompanyStack, getCompanyFit, getSimilarCompanies, getCompanyPostings, getLocations } from '@/lib/api';
import { useSkillStore } from '@/lib/store';
import CompanyLogo from '@/components/CompanyLogo';
import { ResumeDropzone } from '@/components/ResumeDropzone';
import { ArrowLeft, BriefcaseBusiness, CheckCircle2, ChevronDown, ChevronRight, UploadCloud, Bookmark, BookmarkCheck, ArrowRight, ExternalLink, X, Settings, NotebookText } from 'lucide-react';
import Link from 'next/link';
import { Caveat } from 'next/font/google';

const caveat = Caveat({ subsets: ['latin'], weight: ['400', '500', '600', '700'] });

function StackCategory({ category, skills, defaultExpanded, matchedSkillNames, medianIdf, hasResume, pivotSkillId, setPivot, learnList, toggleLearnList }: any) {
  const [expanded, setExpanded] = useState(defaultExpanded);

  const renderSkill = (skill: any, indicatorColor: string) => {
    const isMatched = matchedSkillNames.has(skill.canonical_name);
    const isPivot = pivotSkillId === skill.skill_id;
    const isBookmarked = learnList.some((s: any) => s.skill_id === skill.skill_id);
    
    return (
      <div 
        key={skill.skill_id || skill.name}
        onClick={() => setPivot(skill.skill_id, skill.canonical_name || skill.name)}
        className={`
          relative group flex items-center gap-3 p-[12px] rounded-lg border transition-all cursor-pointer
          ${isMatched 
            ? 'bg-accent-light border-accent-border shadow-sm' 
            : 'bg-bg-subtle border-border hover:border-accent-mid shadow-sm'}
          ${isPivot ? 'ring-2 ring-accent border-accent' : ''}
        `}
      >
        <div className="w-full">
          <div className="flex items-center gap-2 mb-1.5">
            <span className={`text-[13px] font-medium ${isMatched ? 'text-accent' : 'text-text-primary'}`}>
              {skill.canonical_name || skill.name}
            </span>
            {isMatched ? (
              <CheckCircle2 size={14} className="text-accent ml-auto shrink-0" />
            ) : (
              <div className="ml-auto flex items-center gap-2">
                <button 
                  onClick={(e) => { e.stopPropagation(); toggleLearnList({ skill_id: skill.skill_id, canonical_name: skill.canonical_name || skill.name, category }); }}
                  className={`p-1 rounded-md transition-colors ${isBookmarked ? 'text-accent bg-accent/10' : 'text-text-tertiary hover:bg-white/5 hover:text-text-secondary'}`}
                  title={isBookmarked ? "Remove from Learn List" : "Add to Learn List"}
                >
                  <NotebookText size={14} className={isBookmarked ? "fill-accent/20" : ""} />
                </button>
              </div>
            )}
          </div>
          <div className="flex flex-col gap-1 w-full">
            <div className="flex items-center justify-between text-[11px] text-text-tertiary">
              <span>Usage</span>
              <span>{Math.round(skill.signal_strength * 100)}%</span>
            </div>
            <div className="w-full h-1 bg-border rounded-full overflow-hidden flex-shrink-0">
              <div 
                className={`h-full transition-all duration-500 rounded-full ${indicatorColor}`} 
                style={{ width: `${Math.max(4, skill.signal_strength * 100)}%` }}
              />
            </div>
          </div>
        </div>
      </div>
    );
  };

  const matchedCount = skills.filter((s: any) => matchedSkillNames.has(s.canonical_name)).length;

  return (
    <div className="bg-bg-surface border border-border rounded-xl overflow-hidden mb-6 shadow-sm">
      <button 
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between px-4 py-3 bg-bg-subtle hover:bg-border-subtle transition-colors"
      >
        <div className="flex items-center gap-3">
          <h3 className="text-[13px] font-semibold text-text-tertiary uppercase tracking-[0.08em]">{category}</h3>
          {hasResume && matchedCount > 0 && (
            <span className="bg-accent-light text-accent text-[11px] font-semibold px-2 py-0.5 rounded-full flex items-center gap-1 border border-accent-border">
              <CheckCircle2 size={12} />
              {matchedCount}
            </span>
          )}
        </div>
        {expanded ? <ChevronDown size={18} className="text-text-tertiary" /> : <ChevronRight size={18} className="text-text-tertiary" />}
      </button>
      {expanded && (
        <div className="p-4 border-t border-border space-y-6">
          {(() => {
            const highSkills = skills.filter((s: any) => s.signal_strength >= 0.1);
            const midSkills = skills.filter((s: any) => s.signal_strength >= 0.03 && s.signal_strength < 0.1);
            const lowSkills = skills.filter((s: any) => s.signal_strength < 0.03);
            
            return (
              <>
                {highSkills.length > 0 && (
                  <details className="group" open>
                    <summary className="flex items-center gap-2 cursor-pointer outline-none select-none mb-4 text-text-secondary hover:text-text-primary transition-colors">
                      <ChevronRight size={14} className="group-open:rotate-90 transition-transform" />
                      <h4 className="text-[11px] font-semibold uppercase tracking-wider">High Usage</h4>
                      <span className="text-[11px] text-text-tertiary">({highSkills.length})</span>
                    </summary>
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                      {highSkills.map((s: any) => renderSkill(s, 'bg-green-500'))}
                    </div>
                  </details>
                )}
                
                {midSkills.length > 0 && (
                  <details className={`group ${highSkills.length > 0 ? "pt-5 border-t border-border mt-5" : ""}`} open>
                    <summary className="flex items-center gap-2 cursor-pointer outline-none select-none mb-4 text-text-secondary hover:text-text-primary transition-colors">
                      <ChevronRight size={14} className="group-open:rotate-90 transition-transform" />
                      <h4 className="text-[11px] font-semibold uppercase tracking-wider">Medium Usage</h4>
                      <span className="text-[11px] text-text-tertiary">({midSkills.length})</span>
                    </summary>
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                      {midSkills.map((s: any) => renderSkill(s, 'bg-yellow-500'))}
                    </div>
                  </details>
                )}
                
                {lowSkills.length > 0 && (
                  <details className={`group ${(highSkills.length > 0 || midSkills.length > 0) ? "pt-5 border-t border-border mt-5" : ""}`}>
                    <summary className="flex items-center gap-2 cursor-pointer outline-none select-none mb-4 text-text-secondary hover:text-text-primary transition-colors">
                      <ChevronRight size={14} className="group-open:rotate-90 transition-transform" />
                      <h4 className="text-[11px] font-semibold uppercase tracking-wider">Low Usage</h4>
                      <span className="text-[11px] text-text-tertiary">({lowSkills.length})</span>
                    </summary>
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                      {lowSkills.map((s: any) => renderSkill(s, 'bg-red-500'))}
                    </div>
                  </details>
                )}
              </>
            );
          })()}
        </div>
      )}
    </div>
  );
}

export default function CompanyProfilePage({ params }: { params: Promise<{ id: string }> }) {
  const router = useRouter();
  const { id } = use(params);
  const { userSkills, setSkills, total_years_experience, work_types, countries, cities, setTotalYears, setWorkTypes, setCountries, setCities, learnList, toggleLearnList } = useSkillStore();
  
  const [stackData, setStackData] = useState<any>(null);
  const [fitData, setFitData] = useState<any>({});
  const [loading, setLoading] = useState(true);
  const [isUploadModalOpen, setIsUploadModalOpen] = useState(false);
  const [isFilterModalOpen, setIsFilterModalOpen] = useState(false);
  const [locationsData, setLocationsData] = useState<any[]>([]);

  // Split-panel specific state
  const [pivotSkillId, setPivotSkillId] = useState<number | null>(null);
  const [pivotSkillName, setPivotSkillName] = useState<string | null>(null);
  const [postings, setPostings] = useState<any[]>([]);
  const [postingsLoading, setPostingsLoading] = useState(false);
  const [jobSearchQuery, setJobSearchQuery] = useState("");
  const [selectedJobForMissingSkills, setSelectedJobForMissingSkills] = useState<any>(null);

  useEffect(() => {
    getLocations().then(res => setLocationsData(res.countries || [])).catch(console.error);
  }, []);

  useEffect(() => {
    async function loadData() {
      try {
        setLoading(true);
        const stack = await getCompanyStack(id);
        setStackData(stack);

        if (userSkills.length > 0) {
          const skillIds = userSkills.map(s => s.skill_id).filter(Boolean) as number[];
          const fit = await getCompanyFit(id, skillIds);
          setFitData(fit);
        }
      } catch (e) {
        console.error(e);
      } finally {
        setLoading(false);
      }
    }
    loadData();
  }, [id, userSkills]);

  useEffect(() => {
    async function fetchJobs() {
      if (!stackData) return;
      try {
        setPostingsLoading(true);
        const skillIds = userSkills.map(s => s.skill_id).filter(Boolean).join(',');
        
        // We use strict user filters to show only jobs matching their experience/locations
        const res = await getCompanyPostings(
          id, 
          skillIds, 
          total_years_experience, 
          work_types, 
          countries, 
          cities, 
          pivotSkillId
        );
        setPostings(res.postings || []);
      } catch (e) {
        console.error(e);
      } finally {
        setPostingsLoading(false);
      }
    }
    fetchJobs();
  }, [id, pivotSkillId, userSkills, stackData, total_years_experience, work_types, countries, cities]);

  const handleResumeSuccess = async (data: any) => {
    const formatted: any[] = data.skills.map((s: any) => ({
      skill_id: s.skill_id,
      skill_name: s.skill_name,
      canonical_name: s.canonical_name,
      category: s.category,
      confidence: s.confidence,
      evidence: s.evidence
    }));
    setSkills(formatted);
    setIsUploadModalOpen(false);

    const skillIds = formatted.map(s => s.skill_id).filter(Boolean);
    if (skillIds.length > 0) {
      try {
        const fit = await getCompanyFit(id, skillIds);
        setFitData(fit);
      } catch (e) {}
    }
  };

  const handleSetPivot = (skillId: number, skillName: string) => {
    if (pivotSkillId === skillId) {
      setPivotSkillId(null);
      setPivotSkillName(null);
    } else {
      setPivotSkillId(skillId);
      setPivotSkillName(skillName);
    }
  };

  if (loading) return (
    <div className="min-h-screen pt-24 pb-12 flex justify-center">
      <div className="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin mt-12"></div>
    </div>
  );

  if (!stackData) return <div className="min-h-screen pt-24 pb-12 text-center text-text-secondary">Company not found</div>;

  const allSkills = Object.values(stackData.stack || {}).flat() as any[];
  const idfScores = allSkills.map(s => s.idf_score).sort((a, b) => a - b);
  const medianIdf = idfScores.length > 0 ? idfScores[Math.floor(idfScores.length / 2)] : 0;
  const matchedSkillNames = new Set(fitData?.matched_skills || []);
  const pivotUserHas = pivotSkillName ? matchedSkillNames.has(pivotSkillName) : null;

  const stackEntries = Object.entries(stackData.stack || {}).map(([cat, skills]: [string, any]) => {
    const sortedSkills = [...skills].sort((a, b) => b.jd_count - a.jd_count);
    const totalActiveJdCount = skills.reduce((sum: number, s: any) => sum + s.jd_count, 0);
    return { category: cat, skills: sortedSkills, totalActiveJdCount };
  });

  stackEntries.sort((a, b) => b.totalActiveJdCount - a.totalActiveJdCount);

  const thresholdDays = parseInt(process.env.NEXT_PUBLIC_ACTIVE_THRESHOLD_DAYS || process.env.ACTIVE_THRESHOLD_DAYS || '30', 10);
  const months = Math.round(thresholdDays / 30) || 1;

  const displayedPostings = postings.filter(p => p.title.toLowerCase().includes(jobSearchQuery.toLowerCase()));
  const highlyCompatibleCount = displayedPostings.filter(p => p.match_ratio >= 0.5).length;
  const highlyCompatibleText = displayedPostings.length === 20 && highlyCompatibleCount === 20 ? '20+' : highlyCompatibleCount;

  return (
    <div className="max-w-[1400px] mx-auto w-full p-4 md:p-6 pt-24 pb-32 z-10 relative">
      <button 
        onClick={() => router.push('/')}
        className="flex items-center gap-2 text-sm text-muted hover:text-foreground transition-colors mb-6 md:mb-8 group"
      >
        <ArrowLeft size={16} className="group-hover:-translate-x-1 transition-transform" /> Back to Search
      </button>

      <div className="flex flex-col md:flex-row md:items-center justify-between gap-6 mb-8 bg-bg-surface p-6 rounded-xl border border-border shadow-sm">
        <div className="flex items-center gap-6">
          <CompanyLogo domain={stackData.website} name={stackData.name} size={48} />
          <div>
            <h1 className="text-[22px] font-semibold text-text-primary mb-2">{stackData.name}</h1>
            <div className="flex flex-wrap items-center gap-3">
              <div className="flex items-center gap-1.5 text-text-secondary bg-bg-subtle px-3 py-1 rounded-full border border-border text-[12px]">
                <BriefcaseBusiness size={14} />
                {stackData.active_jd_count.toLocaleString()} postings since {months} {months === 1 ? 'month' : 'months'}
              </div>
            </div>
          </div>
        </div>
        
        <div className="flex gap-4">
          {userSkills.length > 0 && fitData ? (
            <div className="bg-bg-subtle p-4 rounded-xl border border-border flex flex-col justify-center gap-3 min-w-[280px]">
              <div className="flex items-center gap-3">
                <div className="w-6 h-6 rounded-full bg-match-light flex items-center justify-center shrink-0 text-match">
                  <CheckCircle2 size={14} />
                </div>
                <div className="text-sm text-text-primary">
                  <strong className="text-match">{fitData.matched_skills?.length || 0}</strong> of <strong>{userSkills.length}</strong> skills matched
                </div>
              </div>
              <div className="flex items-center gap-3">
                <div className="w-6 h-6 rounded-full bg-accent-light flex items-center justify-center shrink-0 text-accent">
                  <BriefcaseBusiness size={14} />
                </div>
                <div className="text-sm text-text-primary">
                  {postingsLoading ? (
                    <span className="text-text-tertiary">Calculating...</span>
                  ) : (
                    <>
                      <strong className="text-accent">{highlyCompatibleText}</strong> highly compatible jobs
                    </>
                  )}
                </div>
              </div>
            </div>
          ) : (
            <button 
              onClick={() => setIsUploadModalOpen(true)}
              className="bg-accent hover:bg-accent-mid text-white px-4 py-2 rounded-lg flex items-center gap-2 transition-colors shrink-0"
            >
              <UploadCloud size={16} />
              <span className="text-[14px] font-medium">Upload Resume for Matches</span>
            </button>
          )}
        </div>
      </div>

      <div className="flex flex-col md:flex-row gap-6 md:gap-8 items-start">
        {/* Left Panel: Stack Categories */}
        <div className="w-full md:w-[60%] flex flex-col">

          <div className="space-y-6 w-full">
            {stackEntries.map((entry, idx) => (
              <StackCategory 
                key={entry.category}
                category={entry.category}
                skills={entry.skills}
                defaultExpanded={idx < 3}
                matchedSkillNames={matchedSkillNames}
                medianIdf={medianIdf}
                hasResume={userSkills.length > 0}
                pivotSkillId={pivotSkillId}
                setPivot={handleSetPivot}
                learnList={learnList}
                toggleLearnList={toggleLearnList}
              />
            ))}
          </div>
        </div>

        {/* Right Panel: Job Explorer */}
        <div className="w-full md:w-[40%] md:sticky md:top-6 flex flex-col gap-4 max-h-[calc(100vh-3rem)]">
          <div className="bg-bg-surface border border-border rounded-xl shadow-sm flex flex-col h-full overflow-hidden">
            <div className="p-5 border-b border-border bg-bg-subtle shrink-0">
              <h2 className="text-[16px] font-semibold text-text-primary">
                {!pivotSkillId 
                  ? "Compatible Jobs" 
                  : pivotUserHas 
                    ? `Jobs needing ${pivotSkillName}` 
                    : `Jobs if you learn ${pivotSkillName}`}
              </h2>

              {/* Active Filters Display */}
              <div className="mt-3 flex flex-wrap gap-2 items-center">
                {total_years_experience > 0 && (
                  <span className="text-[11px] px-2 py-1 rounded-full bg-bg-surface border border-border text-text-secondary flex items-center gap-1.5 font-medium group">
                    <BriefcaseBusiness size={12} />
                    {total_years_experience} {total_years_experience === 1 ? 'yr' : 'yrs'} exp
                    <button onClick={() => setTotalYears(0)} className="ml-1 opacity-50 hover:opacity-100 hover:text-accent transition-all">
                      <X size={12} />
                    </button>
                  </span>
                )}
                {work_types.map((type: string) => (
                  <span key={type} className="text-[11px] px-2 py-1 rounded-full bg-bg-surface border border-border text-text-secondary font-medium flex items-center gap-1 group">
                    {type}
                    <button onClick={() => setWorkTypes(work_types.filter((t: string) => t !== type))} className="ml-0.5 opacity-50 hover:opacity-100 hover:text-accent transition-all">
                      <X size={12} />
                    </button>
                  </span>
                ))}
                {countries.map((country: string) => (
                  <span key={country} className="text-[11px] px-2 py-1 rounded-full bg-bg-surface border border-border text-text-secondary font-medium flex items-center gap-1 group">
                    {country}
                    <button onClick={() => setCountries(countries.filter((c: string) => c !== country))} className="ml-0.5 opacity-50 hover:opacity-100 hover:text-accent transition-all">
                      <X size={12} />
                    </button>
                  </span>
                ))}
                {cities.map((city: string) => (
                  <span key={city} className="text-[11px] px-2 py-1 rounded-full bg-bg-surface border border-border text-text-secondary font-medium flex items-center gap-1 group">
                    {city}
                    <button onClick={() => setCities(cities.filter((c: string) => c !== city))} className="ml-0.5 opacity-50 hover:opacity-100 hover:text-accent transition-all">
                      <X size={12} />
                    </button>
                  </span>
                ))}
                {total_years_experience === 0 && work_types.length === 0 && countries.length === 0 && cities.length === 0 && userSkills.length > 0 && (
                  <span className="text-[11px] text-text-tertiary italic py-1">No active filters</span>
                )}
                <button 
                  onClick={() => setIsFilterModalOpen(true)}
                  className="text-[11px] px-2 py-1 rounded-full bg-accent/10 text-accent hover:bg-accent/20 transition-colors font-medium flex items-center gap-1 ml-auto"
                >
                  <Settings size={12} /> Edit Filters
                </button>
              </div>

              <div className="mt-4">
                <input 
                  type="text" 
                  placeholder="Filter jobs by title..."
                  value={jobSearchQuery}
                  onChange={(e) => setJobSearchQuery(e.target.value)}
                  className="w-full bg-bg-subtle border border-border text-text-primary text-sm rounded-lg px-3 py-2 focus:outline-none focus:border-accent transition-colors"
                />
              </div>
            </div>

            <div className="flex-1 overflow-y-auto p-5 relative">
              {postingsLoading ? (
                <div className="flex justify-center items-center py-12">
                  <div className="w-6 h-6 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
                </div>
              ) : displayedPostings.length === 0 ? (
                <div className="text-center py-12 px-4">
                  {userSkills.length === 0 && !pivotSkillId ? (
                    <>
                      <p className="text-text-secondary text-sm mb-4">Upload your resume to see compatible jobs</p>
                      <button 
                        onClick={() => setIsUploadModalOpen(true)}
                        className="mx-auto bg-bg-subtle border border-border text-text-primary px-4 py-2 rounded-lg text-sm hover:bg-border transition-colors"
                      >
                        Upload Resume
                      </button>
                    </>
                  ) : userSkills.length > 0 && !pivotSkillId ? (
                    <>
                      <p className="text-text-secondary text-sm">No compatible roles match your current filters.</p>
                      {(total_years_experience > 0 || work_types.length > 0) && (
                        <p className="text-text-tertiary text-[12px] mt-2">Try adjusting your experience level or location preferences.</p>
                      )}
                    </>
                  ) : (
                    <p className="text-text-secondary text-sm">No active roles require {pivotSkillName}</p>
                  )}
                </div>
              ) : (
                <div className="space-y-4">
                  {displayedPostings.map(p => {
                    const isHighlyCompatible = p.match_ratio >= 0.5;
                    const matchedRatioPct = Math.round(p.match_ratio * 100);
                    
                    const jobMissingSkills = [...(p.missing_skills || [])];
                    if (pivotSkillName && !pivotUserHas && !jobMissingSkills.includes(pivotSkillName)) {
                      jobMissingSkills.unshift(pivotSkillName);
                    }
                    const jobMatchedSkills = [...(p.matched_skills || [])];
                    if (pivotSkillName && pivotUserHas && !jobMatchedSkills.includes(pivotSkillName)) {
                      jobMatchedSkills.unshift(pivotSkillName);
                    }
                    
                    return (
                      <div key={p.id} className={`p-4 rounded-lg bg-bg-subtle border transition-colors ${isHighlyCompatible ? 'border-accent/40 border-l-[3px] shadow-sm hover:border-accent' : 'border-border hover:border-accent-border'}`}>
                        <div className="flex justify-between items-start gap-4 mb-2">
                          <h4 className="text-[14px] font-medium text-text-primary leading-snug">{p.title}</h4>
                          <a 
                            href={p.apply_url} 
                            target="_blank" 
                            rel="noreferrer"
                            className="text-[12px] font-medium text-accent hover:text-accent-mid whitespace-nowrap flex items-center gap-1 bg-accent/10 px-2 py-1 rounded"
                          >
                            Apply <ExternalLink size={12} />
                          </a>
                        </div>
                        
                        {userSkills.length > 0 && (
                          <div className="flex items-center gap-2 mb-3">
                            <div className="flex-1 h-1.5 bg-border rounded-full overflow-hidden">
                              <div className={`h-full rounded-full transition-all ${p.match_ratio >= 0.5 ? 'bg-green-500' : p.match_ratio >= 0.25 ? 'bg-yellow-500' : 'bg-red-500'}`} style={{ width: `${matchedRatioPct}%` }} />
                            </div>
                            <span className="text-[11px] font-semibold text-text-secondary w-12 text-right">{matchedRatioPct}% match</span>
                          </div>
                        )}

                        <div className="flex flex-wrap items-center gap-2">
                          {p.min_years_required != null && (
                            <span className="text-[11px] px-2 py-0.5 rounded-full bg-white/5 border border-border text-text-secondary font-medium">
                              {p.min_years_required}+ yrs
                            </span>
                          )}
                          {pivotSkillName && (
                            <span className="text-[11px] px-2 py-0.5 rounded-full bg-accent/10 text-accent font-medium">
                              {pivotSkillName}
                            </span>
                          )}
                          {p.matched_skills?.slice(0, 4).map((sk: string) => (
                            <span key={sk} className="text-[11px] px-2 py-0.5 rounded-full bg-accent/10 text-accent font-medium">
                              {sk}
                            </span>
                          ))}
                          {p.matched_skills?.length > 4 && (
                            <span className="text-[11px] px-2 py-0.5 rounded-full bg-bg-surface border border-border text-text-tertiary">
                              +{p.matched_skills.length - 4} more
                            </span>
                          )}
                          {jobMissingSkills.length > 0 && (
                            <button 
                              onClick={() => setSelectedJobForMissingSkills({...p, computed_missing: jobMissingSkills, computed_matched: jobMatchedSkills})}
                              className="text-[11px] font-medium text-text-secondary hover:text-text-primary hover:underline ml-auto"
                            >
                              View missing skills
                            </button>
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {isUploadModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
          <div className="absolute inset-0" onClick={() => setIsUploadModalOpen(false)} />
          <div className="relative bg-bg-surface border border-border w-full max-w-2xl rounded-2xl shadow-md p-8">
            <h2 className="text-2xl font-bold mb-2 text-text-primary">Upload Resume</h2>
            <p className="text-text-secondary mb-6">Let Gemini extract your skills to personalize your matches.</p>
            <ResumeDropzone 
              onSuccess={handleResumeSuccess} 
              onError={(err: any) => alert(err)} 
            />
          </div>
        </div>
      )}

      {/* Filter Modal */}
      {isFilterModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
          <div className="bg-bg-surface w-full max-w-md rounded-2xl shadow-xl overflow-hidden animate-in fade-in zoom-in-95 duration-200">
            <div className="flex items-center justify-between p-5 border-b border-border">
              <h3 className="text-lg font-semibold text-text-primary">Edit Job Filters</h3>
              <button 
                onClick={() => setIsFilterModalOpen(false)}
                className="text-text-tertiary hover:text-text-primary transition-colors p-1"
              >
                <X size={20} />
              </button>
            </div>
            
            <div className="p-5 space-y-5">
              <div>
                <label className="block text-[13px] font-medium text-text-secondary mb-1.5">Years of Experience</label>
                <input 
                  type="number" 
                  min="0"
                  value={total_years_experience}
                  onChange={(e) => setTotalYears(parseInt(e.target.value) || 0)}
                  className="w-full bg-bg-subtle border border-border rounded-lg px-3 py-2 text-sm text-text-primary focus:outline-none focus:border-accent"
                />
              </div>
              
              <FilterSelect 
                label="Work Types" 
                options={["Remote", "Hybrid", "Onsite"]} 
                selected={work_types} 
                onChange={setWorkTypes} 
              />

              <FilterSelect 
                label="Countries" 
                options={Array.from(new Set(locationsData.map((c: any) => c.name))).sort()} 
                selected={countries} 
                onChange={setCountries} 
              />

              <FilterSelect 
                label="Cities" 
                options={Array.from(new Set(locationsData
                  .filter((c: any) => countries.length === 0 || countries.includes(c.name))
                  .flatMap((c: any) => c.cities))).sort()} 
                selected={cities} 
                onChange={setCities} 
              />
            </div>
            
            <div className="p-5 border-t border-border bg-bg-subtle flex justify-end">
              <button 
                onClick={() => setIsFilterModalOpen(false)}
                className="bg-accent hover:bg-accent-mid text-white px-5 py-2 rounded-lg text-sm font-medium transition-colors"
              >
                Apply Filters
              </button>
            </div>
          </div>
        </div>
      )}
      {/* Missing Skills Modal */}
      {selectedJobForMissingSkills && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-background/80 backdrop-blur-sm">
          <div className="bg-bg-surface border border-border rounded-xl shadow-lg w-full max-w-md overflow-hidden flex flex-col max-h-[85vh]">
            <div className="flex items-center justify-between p-4 border-b border-border bg-bg-subtle shrink-0">
              <div className="flex-1 pr-4">
                <h3 className="text-[14px] font-semibold text-text-primary leading-snug flex flex-wrap items-center gap-2">
                  <a href={selectedJobForMissingSkills.apply_url} target="_blank" rel="noreferrer" className="hover:text-accent hover:underline flex items-center gap-1">
                    {selectedJobForMissingSkills.title} <ExternalLink size={14} />
                  </a>
                  {selectedJobForMissingSkills.min_years_required != null && (
                    <span className="text-[11px] px-2 py-0.5 rounded-full bg-white/5 border border-border text-text-secondary font-medium whitespace-nowrap">
                      {selectedJobForMissingSkills.min_years_required}+ yrs
                    </span>
                  )}
                </h3>
              </div>
              <button 
                onClick={() => setSelectedJobForMissingSkills(null)}
                className="text-text-tertiary hover:text-text-primary transition-colors p-1"
              >
                <X size={18} />
              </button>
            </div>
            
            <div className="p-5 overflow-y-auto flex flex-col gap-6">
              <div>
                <h4 className="text-[13px] font-medium text-text-secondary mb-3 flex items-center gap-2">
                  <CheckCircle2 size={14} className="text-match" /> 
                  Skills you have
                </h4>
                <div className="flex flex-wrap gap-2">
                  {selectedJobForMissingSkills.computed_matched?.length > 0 ? (
                    selectedJobForMissingSkills.computed_matched.map((sk: string) => (
                      <span key={sk} className="text-[12px] px-2.5 py-1 rounded-full bg-match/10 border border-match/20 text-match font-medium">
                        {sk}
                      </span>
                    ))
                  ) : (
                    <span className="text-[12px] text-text-tertiary italic">None matched</span>
                  )}
                </div>
              </div>

              <div>
                <h4 className="text-[13px] font-medium text-text-secondary mb-3 flex items-center gap-2">
                  <X size={14} className="text-red-500" /> 
                  Skills you're missing
                </h4>
                <div className="flex flex-wrap gap-2">
                  {selectedJobForMissingSkills.computed_missing?.length > 0 ? (
                    selectedJobForMissingSkills.computed_missing.map((sk: string) => (
                      <span key={sk} className="text-[12px] px-2.5 py-1 rounded-full bg-red-500/10 border border-red-500/20 text-red-500 font-medium">
                        {sk}
                      </span>
                    ))
                  ) : (
                    <span className="text-[12px] text-text-tertiary italic">No missing skills!</span>
                  )}
                </div>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

const FilterSelect = ({ label, options, selected, onChange }: { label: string, options: string[], selected: string[], onChange: (v: string[]) => void }) => {
  return (
    <div>
      <label className="block text-[13px] font-medium text-text-secondary mb-1.5">{label}</label>
      {selected.length > 0 && (
        <div className="flex flex-wrap gap-2 mb-2">
          {selected.map(val => (
            <span key={val} className="flex items-center gap-1 px-2 py-1 bg-bg-surface border border-border rounded-full text-text-secondary text-[12px] font-medium">
              {val}
              <button onClick={() => onChange(selected.filter(v => v !== val))} className="text-text-tertiary hover:text-accent p-0.5">
                <X size={10} />
              </button>
            </span>
          ))}
        </div>
      )}
      <div className="relative">
        <select 
          className="w-full bg-bg-subtle border border-border rounded-lg px-3 py-2 text-sm text-text-primary focus:outline-none focus:border-accent appearance-none cursor-pointer"
          onChange={(e) => {
            if (e.target.value && !selected.includes(e.target.value)) {
              onChange([...selected, e.target.value]);
            }
            e.target.value = ""; // reset
          }}
          value=""
        >
          <option value="" disabled>Select {label.toLowerCase()}...</option>
          {options.filter(opt => !selected.includes(opt)).map(opt => (
            <option key={opt} value={opt}>{opt}</option>
          ))}
        </select>
        <div className="absolute inset-y-0 right-3 flex items-center pointer-events-none">
          <ChevronDown size={14} className="text-text-tertiary" />
        </div>
      </div>
    </div>
  );
};
