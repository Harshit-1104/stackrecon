'use client';

import React, { useEffect, useRef, useMemo, useState } from 'react';
import * as d3 from 'd3';
import { useSkillStore } from '@/lib/store';

interface SkillGraphProps {
  stack: Record<string, any[]>;
}

interface NodeData extends d3.SimulationNodeDatum {
  id: string;
  name: string;
  signal_strength: number;
  jd_count: number;
  active: boolean;
  category: string;
  isMatched: boolean;
  radius: number;
}

interface EdgeData extends d3.SimulationLinkDatum<NodeData> {
  source: string;
  target: string;
}

export function SkillGraph({ stack }: SkillGraphProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const svgRef = useRef<SVGSVGElement>(null);
  const { userSkills } = useSkillStore();
  
  const [tooltip, setTooltip] = useState<{
    show: boolean;
    x: number;
    y: number;
    data?: NodeData;
  }>({ show: false, x: 0, y: 0 });

  const userSkillIds = useMemo(() => {
    return new Set(userSkills.map(s => s.skill_id).filter(Boolean));
  }, [userSkills]);

  useEffect(() => {
    if (!containerRef.current || !svgRef.current) return;

    const width = containerRef.current.clientWidth;
    const height = Math.max(600, containerRef.current.clientHeight);

    const svg = d3.select(svgRef.current);
    svg.selectAll("*").remove(); // Clear previous render

    const nodes: NodeData[] = [];
    const links: EdgeData[] = [];
    
    // Generate Category Centers for gravity
    const categories = Object.keys(stack);
    const catCenters: Record<string, {x: number, y: number}> = {};
    const cols = Math.ceil(Math.sqrt(categories.length));
    categories.forEach((cat, i) => {
      catCenters[cat] = {
        x: (i % cols + 0.5) * (width / cols),
        y: (Math.floor(i / cols) + 0.5) * (height / Math.ceil(categories.length / cols))
      };
    });

    Object.entries(stack).forEach(([category, skills]) => {
      const catNodes = skills.map((skill) => {
        const isMatched = userSkillIds.has(skill.skill_id);
        const radius = 20 + (skill.signal_strength * 30);
        return {
          id: `node-${skill.skill_id}`,
          name: skill.canonical_name,
          signal_strength: skill.signal_strength,
          jd_count: skill.jd_count,
          active: skill.active,
          category,
          isMatched,
          radius
        } as NodeData;
      });
      
      nodes.push(...catNodes);

      // Connect all within category as a chain or star to keep them grouped
      for (let i = 0; i < catNodes.length - 1; i++) {
        links.push({
          source: catNodes[i].id,
          target: catNodes[i+1].id
        });
      }
    });

    // Setup simulation
    const simulation = d3.forceSimulation<NodeData>(nodes)
      .force("link", d3.forceLink<NodeData, EdgeData>(links).id(d => d.id).distance(80))
      .force("charge", d3.forceManyBody().strength(-300))
      .force("collide", d3.forceCollide().radius(d => (d as NodeData).radius + 5).iterations(2))
      .force("x", d3.forceX<NodeData>().x(d => catCenters[d.category]?.x || width/2).strength(0.05))
      .force("y", d3.forceY<NodeData>().y(d => catCenters[d.category]?.y || height/2).strength(0.05))
      .force("center", d3.forceCenter(width / 2, height / 2).strength(0.01));

    const g = svg.append("g");

    // Zoom setup
    const zoom = d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.1, 4])
      .on("zoom", (event) => {
        g.attr("transform", event.transform);
      });
    
    svg.call(zoom as any);

    // Draw Links
    const link = g.append("g")
      .attr("stroke", "#2a2a3e")
      .attr("stroke-opacity", 0.4)
      .attr("stroke-width", 2)
      .selectAll("line")
      .data(links)
      .join("line");

    // Draw Nodes
    const node = g.append("g")
      .selectAll("g")
      .data(nodes)
      .join("g")
      .call(d3.drag<any, NodeData>()
        .on("start", (event, d) => {
          if (!event.active) simulation.alphaTarget(0.3).restart();
          d.fx = d.x;
          d.fy = d.y;
        })
        .on("drag", (event, d) => {
          d.fx = event.x;
          d.fy = event.y;
        })
        .on("end", (event, d) => {
          if (!event.active) simulation.alphaTarget(0);
          d.fx = null;
          d.fy = null;
        })
      )
      .on("mouseover", (event, d) => {
        setTooltip({
          show: true,
          x: event.clientX,
          y: event.clientY,
          data: d
        });
        d3.select(event.currentTarget).select("circle").attr("filter", "brightness(1.2)");
      })
      .on("mousemove", (event) => {
        setTooltip(prev => ({ ...prev, x: event.clientX, y: event.clientY }));
      })
      .on("mouseout", (event) => {
        setTooltip(prev => ({ ...prev, show: false }));
        d3.select(event.currentTarget).select("circle").attr("filter", null);
      });

    node.append("circle")
      .attr("r", d => d.radius)
      .attr("fill", d => d.isMatched ? "#6366f1" : "#1e1e2e")
      .attr("stroke", d => d.isMatched ? "none" : "#3f3f46")
      .attr("stroke-width", d => d.isMatched ? 0 : 2)
      .attr("stroke-dasharray", d => d.isMatched ? "none" : "4,4")
      .attr("class", "transition-colors duration-200 cursor-pointer");

    node.append("text")
      .text(d => d.name)
      .attr("text-anchor", "middle")
      .attr("dy", "0.3em")
      .attr("fill", d => d.isMatched ? "#ffffff" : "#a1a1aa") // white or muted
      .style("font-size", d => Math.max(10, d.radius * 0.4) + "px")
      .style("pointer-events", "none")
      .style("font-weight", d => d.isMatched ? "600" : "400");

    simulation.on("tick", () => {
      link
        .attr("x1", d => (d.source as any).x)
        .attr("y1", d => (d.source as any).y)
        .attr("x2", d => (d.target as any).x)
        .attr("y2", d => (d.target as any).y);

      node.attr("transform", d => `translate(${d.x},${d.y})`);
    });

    return () => {
      simulation.stop();
    };
  }, [stack, userSkillIds]);

  return (
    <div className="w-full h-[600px] border border-border rounded-xl overflow-hidden bg-background relative" ref={containerRef}>
      <svg ref={svgRef} className="w-full h-full cursor-grab active:cursor-grabbing" />
      
      {/* Tooltip */}
      {tooltip.show && tooltip.data && (
        <div 
          className="fixed pointer-events-none z-50 bg-surface/90 backdrop-blur border border-border px-3 py-2 rounded-lg shadow-xl"
          style={{ left: tooltip.x + 15, top: tooltip.y + 15 }}
        >
          <p className="font-medium text-foreground">{tooltip.data.name}</p>
          <div className="text-xs text-muted mt-1 space-y-1">
            <p>Signal Strength: {Math.round(tooltip.data.signal_strength * 100)}%</p>
            <p>Active JDs: {tooltip.data.jd_count}</p>
            <p>Status: {tooltip.data.active ? 'Active' : 'Legacy'}</p>
          </div>
        </div>
      )}

      {/* Legend */}
      <div className="absolute bottom-4 right-4 bg-surface/80 backdrop-blur border border-border p-3 rounded-lg shadow-xl text-xs space-y-2 select-none">
        <div className="flex items-center gap-2">
          <div className="w-4 h-4 rounded-full bg-accent"></div>
          <span className="text-foreground">Matched Skill</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-4 h-4 rounded-full bg-[#1e1e2e] border-2 border-[#3f3f46] border-dashed"></div>
          <span className="text-muted">Gap Skill</span>
        </div>
        <div className="text-muted mt-2 pt-2 border-t border-border">
          Size = Signal Strength
        </div>
      </div>
    </div>
  );
}
