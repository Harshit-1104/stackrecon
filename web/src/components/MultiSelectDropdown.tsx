import { useState, useRef, useEffect } from 'react';
import { ChevronDown, Check } from 'lucide-react';

interface MultiSelectDropdownProps {
  label: string;
  options: string[];
  selected: string[];
  onChange: (selected: string[]) => void;
}

export function MultiSelectDropdown({ label, options, selected, onChange }: MultiSelectDropdownProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (ref.current && !ref.current.contains(event.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const toggleOption = (opt: string) => {
    if (selected.includes(opt)) {
      onChange(selected.filter(s => s !== opt));
    } else {
      onChange([...selected, opt]);
    }
  };

  const clear = (e: React.MouseEvent) => {
    e.stopPropagation();
    onChange([]);
  };

  return (
    <div className="relative" ref={ref}>
      <div className="flex items-center gap-2">
        <label className="text-sm font-medium text-foreground">{label}</label>
        <button 
          onClick={() => setOpen(!open)}
          className="flex items-center gap-2 bg-background border border-border rounded-md px-3 py-1.5 text-sm min-w-[120px] justify-between hover:border-accent transition-colors"
        >
          <span className="truncate max-w-[150px]">
            {selected.length === 0 ? "Any" : selected.length === 1 ? selected[0] : `${selected.length} selected`}
          </span>
          <ChevronDown size={14} className="text-muted shrink-0" />
        </button>
      </div>

      {open && (
        <div className="absolute top-full left-0 md:right-0 md:left-auto mt-1 w-64 bg-surface border border-border rounded-lg shadow-xl z-50 max-h-60 overflow-y-auto">
          <div className="p-2">
            {selected.length > 0 && (
              <button 
                onClick={clear}
                className="w-full text-left px-3 py-1.5 text-sm text-accent hover:bg-white/5 rounded-md mb-1"
              >
                Clear selection
              </button>
            )}
            {options.length === 0 ? (
              <div className="px-3 py-2 text-sm text-muted">No options</div>
            ) : (
              options.map(opt => (
                <button
                  key={opt}
                  onClick={() => toggleOption(opt)}
                  className="w-full text-left px-3 py-2 text-sm hover:bg-white/5 rounded-md flex items-center justify-between"
                >
                  <span className="truncate">{opt}</span>
                  {selected.includes(opt) && <Check size={14} className="text-accent shrink-0" />}
                </button>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}
