import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export interface UserSkill {
  skill_id?: number;
  skill_name: string;
  canonical_name?: string;
  category?: string;
  confidence?: string;
  evidence?: string;
}

export interface LearnSkill {
  skill_id: number;
  canonical_name: string;
  category: string;
}

interface SkillStore {
  userSkills: UserSkill[];
  total_years_experience: number;
  work_types: string[];
  countries: string[];
  cities: string[];
  setSkills: (skills: UserSkill[]) => void;
  setTotalYears: (years: number) => void;
  setWorkTypes: (types: string[]) => void;
  setCountries: (countries: string[]) => void;
  setCities: (cities: string[]) => void;
  addSkill: (skill: UserSkill) => void;
  removeSkill: (skillName: string) => void;
  removeSkillById: (id: number) => void;
  learnList: LearnSkill[];
  toggleLearnList: (skill: LearnSkill) => void;
}

export const useSkillStore = create<SkillStore>()(
  persist(
    (set) => ({
      userSkills: [],
      total_years_experience: 0,
      work_types: [],
      countries: [],
      cities: [],
      setSkills: (skills) => set({ userSkills: skills }),
      setTotalYears: (years) => set({ total_years_experience: years }),
      setWorkTypes: (types) => set({ work_types: types }),
      setCountries: (countries) => set({ countries: countries }),
      setCities: (cities) => set({ cities: cities }),
      addSkill: (skill) => set((state) => {
        if (skill.skill_id && state.userSkills.some(s => s.skill_id === skill.skill_id)) return state;
        if (!skill.skill_id && state.userSkills.some(s => s.skill_name === skill.skill_name)) return state;
        return { userSkills: [...state.userSkills, skill] };
      }),
      removeSkill: (skillName) => set((state) => ({
        userSkills: state.userSkills.filter(s => s.skill_name !== skillName && s.canonical_name !== skillName)
      })),
      removeSkillById: (id) => set((state) => ({
        userSkills: state.userSkills.filter(s => s.skill_id !== id)
      })),
      learnList: [],
      toggleLearnList: (skill) => set((state) => {
        const exists = state.learnList.some(s => s.skill_id === skill.skill_id);
        if (exists) {
          return { learnList: state.learnList.filter(s => s.skill_id !== skill.skill_id) };
        } else {
          return { learnList: [...state.learnList, skill] };
        }
      }),
    }),
    {
      name: 'stackrecon-user-skills',
    }
  )
);
