import moderngl
try:
    ctx = moderngl.create_standalone_context(backend='egl')
    print('ModernGL context created successfully')
    ctx.release()
except Exception as e:
    print(f'ModernGL error: {e}')
    exit(1)