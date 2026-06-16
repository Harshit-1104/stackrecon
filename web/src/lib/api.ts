const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8081/api/v0';

export async function parseResume(file: File) {
  const formData = new FormData();
  formData.append('resume', file);
  const res = await fetch(`${API_URL}/resume/parse`, {
    method: 'POST',
    body: formData,
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function searchSkills(q: string) {
  const res = await fetch(`${API_URL}/skills/search?q=${encodeURIComponent(q)}`);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function matchCompanies(skills: any[], activeOnly = true, totalYearsExperience = 0, workTypes: string[] = [], countries: string[] = [], cities: string[] = []) {
  const res = await fetch(`${API_URL}/match/companies`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ 
      skills, 
      active_only: activeOnly, 
      total_years_experience: totalYearsExperience, 
      work_types: workTypes,
      countries: countries,
      cities: cities
    }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function getCompanyStack(id: string | number, skillIds: number[] = []) {
  const query = skillIds.length > 0 ? `?skill_ids=${skillIds.join(',')}` : '';
  const res = await fetch(`${API_URL}/companies/${id}/stack${query}`);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function getCompanyPostings(id: string | number, skillIds: string, minYears: number = 0, workTypes: string[] = [], countries: string[] = [], cities: string[] = [], pivotSkillId: number | null = null) {
  const params = new URLSearchParams();
  if (skillIds) params.append('skill_ids', skillIds);
  if (minYears > 0) params.append('min_years', minYears.toString());
  workTypes.forEach(w => params.append('work_types', w));
  countries.forEach(c => params.append('countries', c));
  cities.forEach(c => params.append('cities', c));
  if (pivotSkillId) params.append('pivot_skill_id', pivotSkillId.toString());
  
  const query = params.toString() ? `?${params.toString()}` : '';
  const res = await fetch(`${API_URL}/companies/${id}/postings${query}`);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function getLocations() {
  const res = await fetch(`${API_URL}/locations`);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function searchCompanies(q: string) {
  const res = await fetch(`${API_URL}/companies/search?q=${encodeURIComponent(q)}`);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function getCompanyFit(id: string | number, skillIds: number[]) {
  const res = await fetch(`${API_URL}/companies/${id}/fit`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ skill_ids: skillIds }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function getSimilarCompanies(id: string | number) {
  const res = await fetch(`${API_URL}/companies/${id}/similar`);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}
