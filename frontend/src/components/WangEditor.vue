<template>
  <div>
    <Editor
      v-model="html"
      :defaultConfig="editorConfig"
      style="border: 1px solid #ccc; min-height: 200px"
    />
    <button @click="sendContent">发送</button>
  </div>
</template>

<script>
import { Editor } from '@wangeditor/editor-for-vue'
import '@wangeditor/editor/dist/css/style.css'

export default {
  components: { Editor },
  data() {
    return {
      html: '',
      editorConfig: {
        placeholder: '请输入内容...',
        MENU_CONF: {
          uploadImage: {
            server: 'http://localhost:8080/upload',
            fieldName: 'image',
            timeout: 5 * 1000,
          }
        }
      }
    }
  },
  methods: {
    sendContent() {
      console.log(this.html)
      // 你可以发给后端: axios.post('/chat', { content: this.html })
    }
  }
}
</script>
