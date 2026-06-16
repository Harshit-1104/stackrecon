import React from 'react';
import { UserSkill } from '@/lib/store';
import { X } from 'lucide-react';
import clsx from 'clsx';

interface SkillChipProps {
  skill: UserSkill;
  onRemove: (name: string) => void;
  unrecognized?: boolean;
}

export function SkillChip({ skill, onRemove, unrecognized }: SkillChipProps) {
  return (
    <div className={clsx(
      "relative inline-flex items-center gap-2 px-3 py-1.5 rounded-md text-sm border transition-colors",
      unrecognized ? "border-red-900 bg-red-950/30 text-red-200" :
      "border-border bg-surface text-foreground hover:border-accent/50"
    )}>
      <span className="font-medium">{skill.canonical_name || skill.skill_name}</span>
      
      <button 
        onClick={() => onRemove(skill.skill_name)}
        className="ml-1 text-muted hover:text-red-400"
      >
        <X size={14} />
      </button>
    </div>
  );
}
