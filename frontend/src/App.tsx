import { useState, useRef, useEffect, useCallback } from 'react';
import './App.css';
import { SelectInputFile, ProcessImage, SelectOutputFile, GetDefaultDPI, LoadImage, DetectCorners } from '../wailsjs/go/main/App';
import { OnFileDrop, OnFileDropOff } from '../wailsjs/runtime/runtime';

interface Point { x: number; y: number; }

type CornerName = 'tl' | 'tr' | 'br' | 'bl';

function App() {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [imgInfo, setImgInfo] = useState<{ w: number; h: number; src: HTMLImageElement } | null>(null);
  const [corners, setCorners] = useState<Record<CornerName, Point>>({
    tl: { x: 0, y: 0 }, tr: { x: 0, y: 0 }, br: { x: 0, y: 0 }, bl: { x: 0, y: 0 },
  });
  const [dpi, setDpi] = useState(350);
  const [status, setStatus] = useState('点击"选择图片"开始');
  const [filePath, setFilePath] = useState('');
  const [dragging, setDragging] = useState<CornerName | null>(null);
  const [displaySize, setDisplaySize] = useState({ w: 800, h: 600 });

  // 加载默认 DPI
  useEffect(() => { GetDefaultDPI().then(setDpi); }, []);

  // 图片加载后计算初始角点（10% 内缩作为起点）
  const initCorners = useCallback((imgW: number, imgH: number) => {
    const margin = 0.1;
    setCorners({
      tl: { x: Math.round(imgW * margin), y: Math.round(imgH * margin) },
      tr: { x: Math.round(imgW * (1 - margin)), y: Math.round(imgH * margin) },
      br: { x: Math.round(imgW * (1 - margin)), y: Math.round(imgH * (1 - margin)) },
      bl: { x: Math.round(imgW * margin), y: Math.round(imgH * (1 - margin)) },
    });
  }, []);

  // 共享的图片加载逻辑
  const loadImageFromPath = useCallback(async (path: string) => {
    try {
      setStatus('加载中...');
      const info = await LoadImage(path);
      if (!info) { setStatus('加载失败'); return; }
      setFilePath(info.filePath);
      const img = new Image();
      img.onload = async () => {
        setImgInfo({ w: info.width, h: info.height, src: img });
        // 尝试自动检测角点，失败则用默认位置
        try {
          const detected = await DetectCorners(path);
          setCorners({
            tl: { x: detected[0].x, y: detected[0].y },
            tr: { x: detected[1].x, y: detected[1].y },
            br: { x: detected[2].x, y: detected[2].y },
            bl: { x: detected[3].x, y: detected[3].y },
          });
          setStatus(`已加载: ${info.width}x${info.height}（自动检测角点）`);
        } catch {
          initCorners(info.width, info.height);
          setStatus(`已加载: ${info.width}x${info.height}（使用默认角点）`);
        }
      };
      img.src = info.base64;
    } catch (e: any) {
      setStatus('错误: ' + e);
    }
  }, [initCorners]);

  // 选择文件
  const handleSelectFile = async () => {
    try {
      setStatus('选择文件中...');
      const info = await SelectInputFile();
      if (!info) { setStatus('已取消'); return; }
      setFilePath(info.filePath);
      const img = new Image();
      img.onload = async () => {
        setImgInfo({ w: info.width, h: info.height, src: img });
        try {
          const detected = await DetectCorners(info.filePath);
          setCorners({
            tl: { x: detected[0].x, y: detected[0].y },
            tr: { x: detected[1].x, y: detected[1].y },
            br: { x: detected[2].x, y: detected[2].y },
            bl: { x: detected[3].x, y: detected[3].y },
          });
          setStatus(`已加载: ${info.width}x${info.height}（自动检测角点）`);
        } catch {
          initCorners(info.width, info.height);
          setStatus(`已加载: ${info.width}x${info.height}（使用默认角点）`);
        }
      };
      img.src = info.base64;
    } catch (e: any) {
      setStatus('错误: ' + e);
    }
  };

  // 计算画布显示尺寸（窗口自适应）
  const updateDisplaySize = useCallback(() => {
    if (!imgInfo) return;
    const container = canvasRef.current?.parentElement;
    if (!container) return;
    const maxW = container.clientWidth - 20;
    const maxH = container.clientHeight - 20;
    const scale = Math.min(maxW / imgInfo.w, maxH / imgInfo.h, 1);
    setDisplaySize({ w: Math.round(imgInfo.w * scale), h: Math.round(imgInfo.h * scale) });
  }, [imgInfo]);

  useEffect(() => {
    updateDisplaySize();
    const onResize = () => updateDisplaySize();
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, [updateDisplaySize]);

  // 绘制画布
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || !imgInfo) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    canvas.width = displaySize.w;
    canvas.height = displaySize.h;
    const scaleX = displaySize.w / imgInfo.w;
    const scaleY = displaySize.h / imgInfo.h;

    // 清除并绘制图片
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.drawImage(imgInfo.src, 0, 0, canvas.width, canvas.height);

    // 绘制角点连线框
    const cs: [Point, Point, Point, Point] = [corners.tl, corners.tr, corners.br, corners.bl];
    ctx.strokeStyle = '#00ff88';
    ctx.lineWidth = 2;
    ctx.setLineDash([6, 3]);
    ctx.beginPath();
    for (let i = 0; i < 4; i++) {
      const p = cs[i];
      ctx[i === 0 ? 'moveTo' : 'lineTo'](p.x * scaleX, p.y * scaleY);
    }
    ctx.closePath();
    ctx.stroke();
    ctx.setLineDash([]);

    // 绘制角点手柄（带标签）
    const labels = ['左上', '右上', '右下', '左下'];
    cs.forEach((p, i) => {
      const sx = p.x * scaleX;
      const sy = p.y * scaleY;

      // 外圈
      ctx.beginPath();
      ctx.arc(sx, sy, 10, 0, Math.PI * 2);
      ctx.fillStyle = '#00ff88';
      ctx.fill();
      ctx.strokeStyle = '#005533';
      ctx.lineWidth = 2;
      ctx.stroke();

      // 内圈
      ctx.beginPath();
      ctx.arc(sx, sy, 5, 0, Math.PI * 2);
      ctx.fillStyle = '#ffffff';
      ctx.fill();

      // 角点标签
      ctx.font = 'bold 12px sans-serif';
      ctx.fillStyle = '#00ff88';
      ctx.textAlign = 'center';
      const labelOffsets = [
        { x: 0, y: -18 },   // 左上：上
        { x: 0, y: -18 },   // 右上：上
        { x: 0, y: 22 },    // 右下：下
        { x: 0, y: 22 },    // 左下：下
      ];
      ctx.fillText(labels[i], sx + labelOffsets[i].x, sy + labelOffsets[i].y);
    });
  }, [imgInfo, corners, displaySize]);

  // 鼠标事件处理
  const getCanvasPos = (e: React.MouseEvent): Point => {
    const canvas = canvasRef.current!;
    const rect = canvas.getBoundingClientRect();
    const scaleX = imgInfo!.w / displaySize.w;
    const scaleY = imgInfo!.h / displaySize.h;
    return {
      x: Math.round((e.clientX - rect.left) * scaleX),
      y: Math.round((e.clientY - rect.top) * scaleY),
    };
  };

  const cornerOrder: CornerName[] = ['tl', 'tr', 'br', 'bl'];

  const handleMouseDown = (e: React.MouseEvent) => {
    if (!imgInfo) return;
    const pos = getCanvasPos(e);
    const threshold = 15;
    for (const name of cornerOrder) {
      const c = corners[name];
      if (Math.abs(c.x - pos.x) < threshold && Math.abs(c.y - pos.y) < threshold) {
        setDragging(name);
        return;
      }
    }
  };

  const handleMouseMove = (e: React.MouseEvent) => {
    if (!dragging || !imgInfo) return;
    const pos = getCanvasPos(e);
    setCorners(prev => ({
      ...prev,
      [dragging]: { x: Math.max(0, Math.min(imgInfo.w - 1, pos.x)), y: Math.max(0, Math.min(imgInfo.h - 1, pos.y)) },
    }));
  };

  const handleMouseUp = () => setDragging(null);

  // 拖放事件处理：使用 Wails 内置 OnFileDrop 获取文件真实路径
  // （File.path 是 Electron/WebKitGTK 特有属性，Windows WebView2 中不存在）
  const [dragOver, setDragOver] = useState(false);

  useEffect(() => {
    OnFileDrop((_x, _y, paths) => {
      if (!paths || paths.length === 0) return;
      setDragOver(false);
      const path = paths[0];
      const ext = path.toLowerCase();
      if (!ext.endsWith('.jpg') && !ext.endsWith('.jpeg') && !ext.endsWith('.png')) {
        setStatus('仅支持 JPG/PNG 格式');
        return;
      }
      loadImageFromPath(path);
    }, false); // useDropTarget=false：整个窗口任意位置放下即触发
    return () => OnFileDropOff();
  }, [loadImageFromPath]);

  // 拖放覆盖层显示/隐藏（仅视觉提示，路径解析交给 Wails OnFileDrop）
  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
  };
  const handleProcess = async () => {
    if (!filePath) { setStatus('请先选择图片'); return; }
    try {
      const base = filePath.replace(/\.[^.]+$/, '');
      const outPath = await SelectOutputFile(base + '_processed.jpg');
      if (!outPath) { setStatus('已取消保存'); return; }

      setStatus('处理中...');
      const orderedCorners: [Point, Point, Point, Point] = [corners.tl, corners.tr, corners.br, corners.bl];
      await ProcessImage(filePath, orderedCorners, dpi, outPath);
      setStatus('处理完成: ' + outPath);
    } catch (e: any) {
      setStatus('处理失败: ' + e);
    }
  };

  return (
    <div className="app" onDragOver={handleDragOver} onDragLeave={handleDragLeave}>
      {dragOver && <div className="drop-overlay"><div>释放以加载图片</div></div>}
      <div className="toolbar">
        <button onClick={handleSelectFile} className="btn">📂 选择图片</button>
        <label className="dpi-label">
          DPI:
          <input type="number" value={dpi} onChange={e => setDpi(Number(e.target.value))}
            min={72} max={1200} className="dpi-input" />
        </label>
        <button onClick={handleProcess} className="btn btn-primary" disabled={!filePath}>
          ✂️ 处理导出
        </button>
        <span className="status">{status}</span>
      </div>

      <div className="canvas-container">
        {imgInfo ? (
          <canvas
            ref={canvasRef}
            onMouseDown={handleMouseDown}
            onMouseMove={handleMouseMove}
            onMouseUp={handleMouseUp}
            onMouseLeave={handleMouseUp}
            style={{ cursor: dragging ? 'grabbing' : 'crosshair' }}
          />
        ) : (
          <div className="placeholder">
            <div className="placeholder-icon">🪪</div>
            <p>点击上方"选择图片"加载身份证照片</p>
            <p className="hint">加载后拖动四个绿色控制点到身份证四角</p>
          </div>
        )}
      </div>
    </div>
  );
}

export default App;
