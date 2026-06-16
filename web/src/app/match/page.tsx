'use client';

import { useEffect, useState } from 'react';
import { matchCompanies, getLocations } from '@/lib/api';
import { useSkillStore } from '@/lib/store';
import { CompanyCard } from '@/components/CompanyCard';
import { MultiSelectDropdown } from '@/components/MultiSelectDropdown';
import { Settings2 } from 'lucide-react';
import Link from 'next/link';

const SkeletonCard = () => (
  <div className="w-[280px] h-[180px] bg-surface/30 border border-border/50 rounded-xl p-5 shrink-0 animate-pulse flex flex-col">
    <div className="flex items-center gap-3 mb-4">
      <div className="w-8 h-8 rounded-md bg-white/5" />
      <div className="h-5 bg-white/5 rounded w-32" />
    </div>
    <div className="flex-1 space-y-2">
      <div className="h-3 bg-white/5 rounded w-full" />
      <div className="h-3 bg-white/5 rounded w-2/3" />
    </div>
    <div className="mt-auto">
      <div className="flex justify-between items-end mb-2">
        <div className="h-4 bg-white/5 rounded w-8" />
        <div className="h-4 bg-white/5 rounded w-16" />
      </div>
      <div className="h-1 bg-white/5 rounded-full w-full" />
    </div>
  </div>
);

export default function MatchPage() {
  const { userSkills, total_years_experience, work_types, countries, cities, setTotalYears, setWorkTypes, setCountries, setCities } = useSkillStore();
  const [results, setResults] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeOnly, setActiveOnly] = useState(true);
  const [error, setError] = useState('');

  const [locationData, setLocationData] = useState<{name: string, cities: string[]}[]>([]);

  // Local state for debouncing
  const [localYoE, setLocalYoE] = useState<number | ''>(total_years_experience || '');

  useEffect(() => {
    getLocations().then(res => setLocationData(res.countries || [])).catch(console.error);
  }, []);

  useEffect(() => {
    const delay = setTimeout(() => {
      setTotalYears(typeof localYoE === 'number' ? localYoE : 0);
    }, 500);
    return () => clearTimeout(delay);
  }, [localYoE, setTotalYears]);

  useEffect(() => {
    const delay = setTimeout(() => {
      fetchMatches();
    }, 300);
    return () => clearTimeout(delay);
  }, [userSkills, activeOnly, total_years_experience, work_types, countries, cities]);

  const fetchMatches = async () => {
    if (userSkills.length === 0) {
      setLoading(false);
      return;
    }
    
    setLoading(true);
    try {
      const validSkills = userSkills
        .filter(s => s.skill_id)
        .map(s => ({
          skill_id: s.skill_id
        }));

      const res = await matchCompanies(validSkills, activeOnly, total_years_experience, work_types, countries, cities);
      setResults(res.results || []);
    } catch (err: any) {
      setError(err.message || 'Failed to fetch matches');
    } finally {
      setLoading(false);
    }
  };

  const availableCountries = locationData.map(c => c.name).sort();
  let availableCities: string[] = [];
  if (countries.length === 0) {
    availableCities = Array.from(new Set(locationData.flatMap(c => c.cities))).sort();
  } else {
    availableCities = Array.from(new Set(locationData.filter(c => countries.includes(c.name)).flatMap(c => c.cities))).sort();
  }

  // Handle cascaded city reset if a selected city is no longer available
  useEffect(() => {
    if (cities.length > 0) {
      const validCities = cities.filter(city => availableCities.includes(city));
      if (validCities.length !== cities.length) {
        setCities(validCities);
      }
    }
  }, [availableCities, cities, setCities]);

  if (userSkills.length === 0) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center p-6 text-center h-screen">
        <p className="text-xl text-muted mb-4">No skills found in your stack.</p>
        <Link href="/" className="text-accent hover:underline">← Go back and add some skills</Link>
      </div>
    );
  }

  const rail1 = results.filter(c => c.rail === 1);
  const rail2 = results.filter(c => c.rail === 2);

  const rail1Max = Math.max(...rail1.map(r => r.qualified_jd_count), 1);
  const rail2Max = Math.max(...rail2.map(r => r.qualified_jd_count), 1);

  const renderRail = (title: string, dotColor: string, companies: any[], maxQualified: number, isMutedLabel = false) => {
    if (!loading && companies.length === 0) return null;

    return (
      <div className="mb-10">
        <div className="flex items-center gap-3 mb-4 px-1">
          <div className={`w-2.5 h-2.5 rounded-full ${dotColor}`} />
          <h2 className={`font-semibold ${isMutedLabel ? 'text-muted' : 'text-foreground'}`}>
            {title}
          </h2>
          {!loading && (
            <span className="px-2 py-0.5 rounded-full bg-white/5 text-xs text-muted font-medium">
              {companies.length}
            </span>
          )}
        </div>

        <div className="flex overflow-x-auto snap-x snap-mandatory gap-4 pb-4 -mx-6 px-6 sm:mx-0 sm:px-0 scrollbar-thin scrollbar-thumb-white/10 hover:scrollbar-thumb-white/20">
          {loading ? (
            <>
              <SkeletonCard />
              <SkeletonCard />
              <SkeletonCard />
            </>
          ) : (
            companies.map(company => (
              <CompanyCard key={company.company_id} company={company} maxQualifiedInRail={maxQualified} />
            ))
          )}
        </div>
      </div>
    );
  };

  return (
    <div className="max-w-6xl mx-auto w-full p-6 pt-12 pb-24 z-10 relative flex flex-col min-h-screen">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-end mb-8 gap-4 shrink-0">
        <div>
          <h1 className="text-3xl font-bold flex items-center gap-3">
            Your Matches
          </h1>
          <p className="text-muted mt-1">Based on {userSkills.length} skills</p>
        </div>
        
        <div className="flex items-center gap-4 bg-surface p-2 rounded-lg border border-border">
          <Link href="/" className="text-sm text-muted hover:text-accent flex items-center gap-1">
            <Settings2 size={14} /> Edit Stack
          </Link>
        </div>
      </div>

      <div className="bg-surface border border-border rounded-xl p-4 mb-10 flex flex-wrap items-center gap-6">
        <MultiSelectDropdown 
          label="Work Type" 
          options={["Remote", "Hybrid", "Onsite"]} 
          selected={work_types} 
          onChange={setWorkTypes} 
        />
        
        <MultiSelectDropdown 
          label="Country" 
          options={availableCountries} 
          selected={countries} 
          onChange={setCountries} 
        />
        
        <MultiSelectDropdown 
          label="City" 
          options={availableCities} 
          selected={cities} 
          onChange={setCities} 
        />
        
        <div className="w-px h-6 bg-border hidden md:block"></div>
        
        <div className="flex items-center gap-3">
          <label className="text-sm font-medium text-foreground">Years of Experience</label>
          <input 
            type="number" 
            min="0" max="40" step="0.5"
            placeholder="0"
            className="bg-background border border-border rounded-md px-3 py-1.5 w-20 text-center focus:outline-none focus:border-accent text-sm"
            value={localYoE}
            onChange={(e) => setLocalYoE(e.target.value === '' ? '' : parseFloat(e.target.value))}
          />
        </div>
      </div>

      {error && <div className="p-4 bg-red-900/20 border border-red-900 text-red-200 rounded-lg mb-8 shrink-0">{error}</div>}

      {!loading && results.length === 0 && !error ? (
        <div className="flex flex-col items-center justify-center py-20 flex-1 text-center">
          <p className="text-xl text-muted mb-4">No matches found. Try adjusting filters or adding more skills.</p>
        </div>
      ) : (
        <div className="flex-1 flex flex-col">
          {renderRail("Actively hiring for your skills", "bg-indigo-500", rail1, rail1Max)}
          {renderRail("Your skills are used here", "bg-zinc-500", rail2, rail2Max, true)}
        </div>
      )}
    </div>
  );
}
