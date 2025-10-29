export async function downloadJson(data: Blob, filename: string) {
    try {  
      // Convert to blob
      const blob = new Blob([JSON.stringify(data, null, 2)], {
        type: 'application/json',
      });
  
      // Create a temporary object URL
      const url = URL.createObjectURL(blob);
  
      // Create a hidden <a> element to trigger the download
      const a = document.createElement('a');
      a.href = url;
      a.download = filename; // filename for the user
      document.body.appendChild(a);
      a.click();
  
      // Clean up
      a.remove();
      URL.revokeObjectURL(url);
    } catch (err) {
      console.error(err);
    }
}