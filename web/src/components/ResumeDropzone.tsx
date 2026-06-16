'use client';

import React, { useCallback, useState } from 'react';
import { useDropzone } from 'react-dropzone';
import { UploadCloud, Loader2 } from 'lucide-react';
import { parseResume } from '@/lib/api';

interface ResumeDropzoneProps {
  onSuccess: (data: any) => void;
  onError: (error: string) => void;
}

export function ResumeDropzone({ onSuccess, onError }: ResumeDropzoneProps) {
  const [loading, setLoading] = useState(false);

  const onDrop = useCallback(async (acceptedFiles: File[]) => {
    if (acceptedFiles.length === 0) return;
    const file = acceptedFiles[0];
    setLoading(true);
    try {
      const data = await parseResume(file);
      onSuccess(data);
    } catch (err: any) {
      onError(err.message || 'Failed to parse resume');
    } finally {
      setLoading(false);
    }
  }, [onSuccess, onError]);

  const { getRootProps, getInputProps, isDragActive } = useDropzone({ 
    onDrop,
    accept: {
      'application/pdf': ['.pdf'],
      'application/vnd.openxmlformats-officedocument.wordprocessingml.document': ['.docx']
    },
    maxFiles: 1
  });

  return (
    <div 
      {...getRootProps()} 
      className={`border-2 border-dashed rounded-xl p-10 flex flex-col items-center justify-center cursor-pointer transition-colors ${
        isDragActive ? 'border-accent bg-accent-light' : 'border-border hover:border-accent bg-bg-surface'
      }`}
    >
      <input {...getInputProps()} />
      {loading ? (
        <>
          <Loader2 className="animate-spin text-accent mb-4" size={40} />
          <p className="text-text-primary font-medium">Parsing your stack...</p>
          <p className="text-sm text-text-tertiary mt-2">Extracting skills</p>
        </>
      ) : (
        <>
          <UploadCloud className="text-accent mb-4" size={40} />
          <p className="text-text-primary font-medium text-lg">
            {isDragActive ? "Drop the file here..." : "Drag & drop your resume"}
          </p>
          <p className="text-sm text-text-tertiary mt-2">Supports PDF and DOCX</p>
        </>
      )}
    </div>
  );
}
