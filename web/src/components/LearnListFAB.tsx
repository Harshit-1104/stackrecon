'use client';

import { useState } from 'react';
import { NotebookText, X } from 'lucide-react';
import { useSkillStore } from '@/lib/store';
import { Caveat } from 'next/font/google';

const caveat = Caveat({ subsets: ['latin'], weight: ['400', '500', '600', '700'] });

export function LearnListFAB() {
  const [isOpen, setIsOpen] = useState(false);
  const { learnList, toggleLearnList } = useSkillStore();


  // Group learn list skills by category
  const learnListByCategory = learnList.reduce((acc: any, skill: any) => {
    const cat = skill.category || "Other";
    if (!acc[cat]) acc[cat] = [];
    acc[cat].push(skill);
    return acc;
  }, {});

  return (
    <>
      {/* FAB */}
      <button
        onClick={() => setIsOpen(true)}
        className="fixed bottom-6 right-6 z-40 bg-accent hover:bg-accent-mid text-white w-14 h-14 rounded-full shadow-[0_4px_14px_rgba(0,0,0,0.25)] flex items-center justify-center transition-transform hover:scale-105"
      >
        <NotebookText size={24} />
        {learnList.length > 0 && (
          <span className="absolute -top-1 -right-1 bg-red-500 text-white text-[10px] font-bold w-5 h-5 flex items-center justify-center rounded-full border-2 border-background">
            {learnList.length}
          </span>
        )}
      </button>

      {/* Notepad Modal */}
      {isOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm p-4">
          <div className="absolute inset-0" onClick={() => setIsOpen(false)} />
          <div className="relative w-full max-w-md shadow-2xl rounded-sm overflow-hidden" style={{ backgroundColor: '#fdf6e3' }}>
            {/* Notebook binding / header */}
            <div className="h-10 bg-red-500/80 flex items-center px-4 justify-between">
              <span className="text-white/90 font-semibold text-sm tracking-wider uppercase">To Learn</span>
              <button onClick={() => setIsOpen(false)} className="text-white hover:text-white/70">
                <X size={18} />
              </button>
            </div>
            
            {/* Notepad body */}
            <div 
              className={`p-6 md:p-8 max-h-[70vh] overflow-y-auto ${caveat.className}`}
              style={{
                backgroundImage: 'repeating-linear-gradient(transparent, transparent 31px, #e0d8c0 31px, #e0d8c0 32px)',
                backgroundSize: '100% 32px',
                backgroundAttachment: 'local',
                lineHeight: '32px'
              }}
            >
              {learnList.length === 0 ? (
                <div className="text-3xl text-text-tertiary mt-4 text-center" style={{ lineHeight: '32px' }}>Nothing to learn yet!</div>
              ) : (
                <div className="space-y-6 pt-1">
                  {Object.entries(learnListByCategory).map(([cat, skills]: [string, any]) => (
                    <div key={cat}>
                      <h3 className="text-3xl font-bold text-accent mb-2" style={{ lineHeight: '32px' }}>{cat}</h3>
                      <ul className="pl-4">
                        {skills.map((sk: any) => (
                          <li key={sk.skill_id} className="text-[28px] text-text-primary flex items-center justify-between group" style={{ lineHeight: '32px' }}>
                            <span>- {sk.canonical_name}</span>
                            <button 
                              onClick={() => toggleLearnList(sk)}
                              className="opacity-0 group-hover:opacity-100 transition-opacity text-red-500/70 hover:text-red-500"
                            >
                              <X size={20} />
                            </button>
                          </li>
                        ))}
                      </ul>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  );
}
